package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"yoinkdl/internal/transfer"
)

func TestNewJobNormalizesWithoutFileWork(t *testing.T) {
	dir := t.TempDir()
	j := testJob(t, dir, 1)
	if *j.Request.URL != "https://example.invalid/file?token=queue-private" || j.State != Queued ||
		j.Published == nil || *j.Published || j.Error != nil || j.Request.Connections != 4 ||
		j.Request.Retries != 3 || j.Request.TimeoutNS != int64(30*time.Second) {
		t.Fatal("job did not capture a normalized queued request")
	}

	req, err := j.Request.Transfer()
	if err != nil || req.Overwrite || req.Output != j.Request.Output || req.URL != *j.Request.URL {
		t.Fatal("queue request changed transfer behavior")
	}

	*req.Retries = 7
	if j.Request.Retries != 3 {
		t.Fatal("transfer request aliases queue settings")
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("job construction performed file work")
	}
}

func TestNewJobRefusesInvalidRequests(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*transfer.Request)
	}{
		{"overwrite", func(r *transfer.Request) { r.Overwrite = true }},
		{"relative", func(r *transfer.Request) { r.Output = "relative" }},
		{"unclean", func(r *transfer.Request) { r.Output += "/../file" }},
		{"reserved", func(r *transfer.Request) { r.Output += ".yoinkdl.part" }},
		{"nul", func(r *transfer.Request) { r.Output += "\x00" }},
		{"path size", func(r *transfer.Request) { r.Output += strings.Repeat("a", MaxPathSize) }},
		{"URL size", func(r *transfer.Request) { r.URL += strings.Repeat("a", MaxURLSize) }},
		{"URL UTF8", func(r *transfer.Request) { r.URL += "\xff" }},
		{"path UTF8", func(r *transfer.Request) { r.Output += "\xff" }},
		{"credentials", func(r *transfer.Request) { r.URL = "https://user:queue-private@example.invalid/file" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := transfer.Request{URL: "https://example.invalid/file?token=queue-private", Output: filepath.Join(t.TempDir(), "file")}
			test.edit(&req)
			_, err := NewJob(strings.Repeat("1", 32), 1, req)
			assertPrivateError(t, err)
		})
	}
}

func TestRequestHashEncoding(t *testing.T) {
	r := Request{
		URL: ptr("https://example.invalid/file?token=private&x=1"), Output: "/output/file",
		Connections: 4, Retries: 3, TimeoutNS: int64(30 * time.Second),
	}
	data, err := json.Marshal(r)
	want := `{"url":"https://example.invalid/file?token=private\u0026x=1","output":"/output/file","connections":4,"retries":3,"timeout_ns":30000000000,"limit_bps":0,"resume":false}`
	if err != nil || string(data) != want {
		t.Fatal("immutable request JSON contract changed")
	}

	if filepath.IsAbs(r.Output) {
		sum, err := r.hash()
		if err != nil || sum != "75e73730f513a8d31e3f039ae077d93de6ccc40fcd6113c5c40863c592165941" {
			t.Fatalf("immutable request hash changed: %v", err)
		}
	}
}

func TestSnapshotRoundTripsAndSourceClearing(t *testing.T) {
	empty := NewSnapshot()
	data, err := encodeSnapshot(empty)
	if err != nil || string(data) != "{\"version\":1,\"parallel\":1,\"next_order\":1,\"jobs\":[]}\n" {
		t.Fatal("empty snapshot contract changed")
	}

	for _, state := range []State{Queued, Running, Pausing, Paused, Canceling, Canceled, Interrupted, Failed, Completed} {
		t.Run(string(state), func(t *testing.T) {
			s := testSnapshot(t, t.TempDir(), 1)
			j := &s.Jobs[0]
			j.State = state
			switch state {
			case Running, Pausing, Canceling, Interrupted:
				j.Published = nil
			case Completed:
				j.Published = ptr(true)
			}

			if state == Interrupted || state == Failed {
				j.Error = ptr("previous attempt failed")
			}

			terminal := state == Completed || state == Canceled
			if terminal {
				j.Request.URL = nil
			}

			data, err := encodeSnapshot(s)
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Contains(data, []byte("queue-private")) == terminal {
				t.Fatal("source retention did not match the lifecycle")
			}

			got, err := decodeSnapshot(data)
			if err != nil || !reflect.DeepEqual(got, s) {
				t.Fatalf("snapshot round trip failed: %v", err)
			}

			if terminal {
				if _, err := got.Jobs[0].Request.Transfer(); err == nil {
					t.Fatal("reconstructed a terminal request after source removal")
				}
			}
		})
	}
}

func TestSnapshotValidation(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Snapshot)
	}{
		{"version", func(s *Snapshot) { s.Version++ }},
		{"parallel low", func(s *Snapshot) { s.Parallel = 0 }},
		{"parallel high", func(s *Snapshot) { s.Parallel = MaxParallel + 1 }},
		{"next order", func(s *Snapshot) { s.NextOrder = 1 }},
		{"nil jobs", func(s *Snapshot) { s.Jobs = nil }},
		{"id", func(s *Snapshot) { s.Jobs[0].ID = "queue-private" }},
		{"id case", func(s *Snapshot) { s.Jobs[0].ID = strings.Repeat("A", 32) }},
		{"order", func(s *Snapshot) { s.Jobs[0].Order = 0 }},
		{"order overflow", func(s *Snapshot) { s.Jobs[0].Order = math.MaxInt64 }},
		{"duplicate id", func(s *Snapshot) { s.Jobs[1].ID = s.Jobs[0].ID }},
		{"duplicate order", func(s *Snapshot) { s.Jobs[1].Order = s.Jobs[0].Order }},
		{"duplicate output", func(s *Snapshot) {
			s.Jobs[1].Request.Output = s.Jobs[0].Request.Output
			s.Jobs[1].InputHash, _ = s.Jobs[1].Request.hash()
		}},
		{"input hash", func(s *Snapshot) { s.Jobs[0].InputHash = strings.Repeat("0", 64) }},
		{"changed request", func(s *Snapshot) { s.Jobs[0].Request.Resume = true }},
		{"source missing", func(s *Snapshot) { s.Jobs[0].Request.URL = nil }},
		{"fragment", func(s *Snapshot) { *s.Jobs[0].Request.URL += "#fragment" }},
		{"default workers", func(s *Snapshot) { s.Jobs[0].Request.Connections = 0 }},
		{"default timeout", func(s *Snapshot) { s.Jobs[0].Request.TimeoutNS = 0 }},
		{"retries", func(s *Snapshot) { s.Jobs[0].Request.Retries = -1 }},
		{"limit", func(s *Snapshot) { s.Jobs[0].Request.LimitBPS = -1 }},
		{"state", func(s *Snapshot) { s.Jobs[0].State = State("queue-private") }},
		{"active result", func(s *Snapshot) { s.Jobs[0].State = Running }},
		{"queued error", func(s *Snapshot) { s.Jobs[0].Error = ptr("problem") }},
		{"interrupted result", func(s *Snapshot) {
			s.Jobs[0].State = Interrupted
			s.Jobs[0].Error = ptr("interrupted")
		}},
		{"failed error", func(s *Snapshot) { s.Jobs[0].State = Failed }},
		{"failed result", func(s *Snapshot) {
			s.Jobs[0].State = Failed
			s.Jobs[0].Error = ptr("failure")
			s.Jobs[0].Published = nil
		}},
		{"terminal source", func(s *Snapshot) { s.Jobs[0].State = Canceled }},
		{"completed result", func(s *Snapshot) {
			s.Jobs[0].State = Completed
			s.Jobs[0].Request.URL = nil
		}},
		{"error size", func(s *Snapshot) {
			s.Jobs[0].State = Failed
			s.Jobs[0].Error = ptr(strings.Repeat("a", MaxErrorSize+1))
		}},
		{"error UTF8", func(s *Snapshot) {
			s.Jobs[0].State = Failed
			s.Jobs[0].Error = ptr("\xff")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testSnapshot(t, t.TempDir(), 2)
			test.edit(&s)
			assertPrivateError(t, s.Validate())
		})
	}
}

func TestSnapshotBoundsAndCloning(t *testing.T) {
	s := testSnapshot(t, t.TempDir(), MaxJobs)
	s.Parallel = MaxParallel
	data, err := encodeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}

	got, err := decodeSnapshot(data)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("full snapshot failed: %v", err)
	}

	copy := s.clone()
	*copy.Jobs[0].Request.URL = "changed"
	*copy.Jobs[0].Published = true
	if *s.Jobs[0].Request.URL == "changed" || *s.Jobs[0].Published {
		t.Fatal("snapshot clone aliases private state")
	}

	s.Jobs = append(s.Jobs, s.Jobs[0])
	assertPrivateError(t, s.Validate())
	s.Jobs = s.Jobs[:MaxJobs]

	for i := range s.Jobs {
		s.Jobs[i].Request.URL = ptr("https://example.invalid/?" + strings.Repeat("&", MaxURLSize-32))
		s.Jobs[i].InputHash, err = s.Jobs[i].Request.hash()
		if err != nil {
			t.Fatal(err)
		}
	}

	assertPrivateError(t, s.Validate())

	exhausted := NewSnapshot()
	exhausted.NextOrder = math.MaxInt64
	if err := exhausted.Validate(); err != nil {
		t.Fatal("valid exhausted counter was refused")
	}
}

func testJob(t *testing.T, dir string, order int64) Job {
	t.Helper()
	j, err := NewJob(fmt.Sprintf("%032x", order), order, transfer.Request{
		URL: "https://example.invalid/file?token=queue-private#fragment", Output: filepath.Join(dir, fmt.Sprintf("file-%d", order)),
	})
	if err != nil {
		t.Fatal(err)
	}

	return j
}

func testSnapshot(t *testing.T, dir string, count int) Snapshot {
	t.Helper()
	s := NewSnapshot()
	for i := range count {
		s.Jobs = append(s.Jobs, testJob(t, dir, int64(i+1)))
	}

	s.NextOrder = int64(count + 1)
	return s
}

func assertPrivateError(t *testing.T, err error) {
	t.Helper()
	if err == nil || strings.Contains(err.Error(), "queue-private") || strings.Contains(err.Error(), "https://") {
		t.Fatalf("missing or unsafe validation error: %v", err)
	}
}

func ptr[T any](value T) *T {
	return &value
}
