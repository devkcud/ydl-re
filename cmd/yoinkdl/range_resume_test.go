package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestProcessParallelCrashResume(t *testing.T) {
	const size = ranges.MinChunkSize
	data := make([]byte, 4*size)
	for i := range data {
		data[i] = byte(i*17 + i/4096)
	}
	wantHash := sha256.Sum256(data)
	bad := bytes.Repeat([]byte("?"), 64*1024)
	var resuming atomic.Bool
	var uncertain, probes atomic.Int32
	var mu sync.Mutex
	var fetched []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resume := resuming.Load()
		var start, end int64
		n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		if n != 2 || err != nil || start < 0 || end < start || end >= int64(len(data)) || r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("invalid process range request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if resume || end != 0 {
			if r.Header.Get("If-Range") != `"parallel-version"` {
				t.Error("process did not retain the selected validator")
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
		}
		w.Header().Set("ETag", `"parallel-version"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)

		if start == 0 && end == 0 {
			if resume {
				probes.Add(1)
			}
			w.Write(data[:1])
			return
		}

		if resume {
			mu.Lock()
			fetched = append(fetched, r.Header.Get("Range"))
			mu.Unlock()
		} else if start != size && start != 3*size {
			w.Write(bad)
			w.(http.Flusher).Flush()
			uncertain.Add(1)
			<-r.Context().Done()
			return
		}

		w.Write(data[start : end+1])
	}))
	defer srv.Close()

	url := srv.URL + "/file?token=parallel-private-token"
	path := filepath.Join(t.TempDir(), "file")
	cmd := command("download", url, "-o", path, "--quiet", "--connections", "4")
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if !finished {
			cmd.Process.Kill()
			<-done
		}
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	ready := false
	for !ready {
		select {
		case <-ticker.C:
			m, err := storage.Inspect(path)
			ready = err == nil && m.Version == storage.RangeManifestVersion && m.State == storage.StatePartial &&
				m.Completed == 2*size && len(m.Ranges) == 2 && uncertain.Load() == 2
			if ready {
				file, err := os.Open(path + storage.PartSuffix)
				if err != nil {
					ready = false
					continue
				}
				buf := make([]byte, len(bad))
				for _, offset := range []int64{0, 2 * size} {
					if _, err := file.ReadAt(buf, offset); err != nil || !bytes.Equal(buf, bad) {
						ready = false
						break
					}
				}
				file.Close()
			}
		case err := <-done:
			finished = true
			t.Fatalf("process stopped before crash injection: %v; %s", err, &errOut)
		case <-deadline.C:
			t.Fatal("process never produced disjoint checkpoints with unfinished workers")
		}
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		finished = true
		if err == nil {
			t.Fatal("killed process unexpectedly succeeded")
		}
	case <-deadline.C:
		t.Fatal("process did not terminate")
	}

	m, err := storage.Inspect(path)
	wantSpans := []ranges.Span{{Start: size, End: 2 * size}, {Start: 3 * size, End: 4 * size}}
	if err != nil || m.ChunkSize != size || !reflect.DeepEqual(m.Spans(), wantSpans) {
		t.Fatalf("invalid disjoint checkpoint: %+v, %v", m, err)
	}
	partial, err := os.ReadFile(path + storage.PartSuffix)
	if err != nil || len(partial) != len(data) || !bytes.HasPrefix(partial, bad) {
		t.Fatalf("missing unconfirmed bytes in a full-sized partial: %v", err)
	}
	meta, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil || bytes.Contains(meta, []byte("parallel-private-token")) || bytes.Contains(meta, []byte(srv.URL)) {
		t.Fatalf("unsafe persisted identity: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("incomplete parallel transfer was published")
	}

	out, err := command("inspect", path).CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte(fmt.Sprintf("Saved bytes: %d", 2*size))) ||
		!bytes.Contains(out, []byte("eligible after local and server validation")) {
		t.Fatalf("inspection after crash = %q, %v", out, err)
	}

	resuming.Store(true)
	out, err = command("download", url, "-o", path, "--quiet", "--resume", "--connections", "2").CombinedOutput()
	if err != nil || string(out) != path+"\n" || probes.Load() != 1 {
		t.Fatalf("parallel resume = %q, error = %v, probes = %d", out, err, probes.Load())
	}
	got, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(got) != wantHash {
		t.Fatalf("resumed process fixture mismatch: %v", err)
	}
	mu.Lock()
	sort.Strings(fetched)
	wantRequests := []string{fmt.Sprintf("bytes=0-%d", size-1), fmt.Sprintf("bytes=%d-%d", 2*size, 3*size-1)}
	sort.Strings(wantRequests)
	if !reflect.DeepEqual(fetched, wantRequests) {
		t.Errorf("fetched %v, want only missing chunks %v", fetched, wantRequests)
	}
	mu.Unlock()
	for _, suffix := range []string{storage.PartSuffix, storage.ManifestSuffix} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("left completed artifact %s: %v", suffix, err)
		}
	}
}
