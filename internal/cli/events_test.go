package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"yoinkdl/internal/transfer"
)

func TestJSONProgressCoalescesAndKeepsExactCounts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var out bytes.Buffer
		e, err := newEvents(&out, false)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		s := transfer.Snapshot{Bytes: math.MaxInt64 - 2, Total: math.MaxInt64, Resumed: math.MaxInt64 - 5, Workers: 4}
		if err := e.Update(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		want := "{\"schema_version\":1,\"event\":\"progress\",\"elapsed_ms\":0,\"bytes\":9223372036854775805,\"total\":9223372036854775807,\"resumed\":9223372036854775802,\"workers\":4}\n"
		if out.String() != want {
			t.Fatalf("progress record = %s", &out)
		}
		s.Bytes++
		if err := e.Update(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		time.Sleep(progressInterval)
		s.Bytes -= 2
		if err := e.Update(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("unbounded progress records: %s", &out)
		}
		var last progressEvent
		if err := json.Unmarshal([]byte(lines[1]), &last); err != nil {
			t.Fatal(err)
		}
		if last.Bytes != s.Bytes || last.Elapsed != 200 || last.Workers != 4 {
			t.Fatalf("retry rollback lost: %+v", last)
		}
	})
}

func TestJSONResultsReportTransferOutcome(t *testing.T) {
	for _, test := range []struct {
		name      string
		published bool
		err       error
		status    string
		code      int
	}{
		{"complete", true, nil, "completed", 0},
		{"failure", false, errors.New("read failed\nwith details"), "failed", 1},
		{"interruption", false, errors.Join(context.Canceled, errors.New("checkpoint failed")), "interrupted", 130},
		{"published with cleanup failure", true, errors.New("manifest cleanup failed"), "failed", 1},
		{"missing publication", false, nil, "failed", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			e, err := newEvents(&out, true)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			if err := e.Update(context.Background(), transfer.Snapshot{Bytes: 7, Total: 10, Resumed: 3, Workers: 2}); err != nil {
				t.Fatal(err)
			}
			if out.Len() != 0 {
				t.Fatal("quiet mode emitted progress")
			}
			path := "/tmp/quoted\"\nfile"
			result := transfer.Result{Path: path, Bytes: 7, Resumed: 3, Published: test.published}
			if test.err != nil {
				result.Partial = path + ".yoinkdl.part"
			}
			code, err := e.Finish(result, test.err)
			if err != nil || code != test.code || bytes.Count(out.Bytes(), []byte("\n")) != 1 {
				t.Fatalf("terminal framing = %q, code=%d, error=%v", &out, code, err)
			}
			var got resultEvent
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Version != 1 || got.Event != "result" || got.Status != test.status || got.ExitCode != test.code ||
				got.Path == nil || *got.Path != path || got.Bytes != 7 || got.Total == nil || *got.Total != 10 ||
				got.Resumed != 3 || got.Published != test.published || (got.Error == nil) != (test.code == 0) {
				t.Fatalf("terminal outcome = %+v", got)
			}
			if test.err != nil && (got.Partial == nil || *got.Partial != result.Partial || *got.Error != test.err.Error()) {
				t.Fatal("terminal result lost preservation or error details")
			}
			before := out.String()
			if _, err := e.Finish(result, test.err); err == nil {
				t.Fatal("accepted a duplicate terminal result")
			}
			e.Update(context.Background(), transfer.Snapshot{})
			if out.String() != before {
				t.Fatal("emitted data after the terminal result")
			}
		})
	}
}

func TestJSONUnknownAndEmptyTotals(t *testing.T) {
	for _, total := range []int64{-1, 0} {
		var out bytes.Buffer
		e, err := newEvents(&out, false)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		if err := e.Update(context.Background(), transfer.Snapshot{Total: total}); err != nil {
			t.Fatal(err)
		}
		var got progressEvent
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if (got.Total == nil) != (total < 0) || (got.Total != nil && *got.Total != 0) {
			t.Fatalf("total %d encoded as %s", total, &out)
		}
	}
	var out bytes.Buffer
	e, err := newEvents(&out, false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	code, err := e.Finish(transfer.Result{}, errors.New("transfer could not open"))
	var got map[string]any
	if err != nil || code != 1 || json.Unmarshal(out.Bytes(), &got) != nil {
		t.Fatalf("early failure = %q, code=%d, error=%v", &out, code, err)
	}
	for _, field := range []string{"path", "partial", "total"} {
		value, ok := got[field]
		if !ok || value != nil {
			t.Fatalf("unavailable field %s = %v", field, value)
		}
	}
}

func TestJSONWriteFailureStopsTheStream(t *testing.T) {
	for _, short := range []bool{false, true} {
		calls := 0
		out := writeFunc(func(p []byte) (int, error) {
			calls++
			if short {
				return len(p) - 1, nil
			}
			return 0, io.ErrClosedPipe
		})
		e, err := newEvents(out, false)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		want := io.ErrClosedPipe
		if short {
			want = io.ErrShortWrite
		}
		if err := e.Update(context.Background(), transfer.Snapshot{Total: -1}); !errors.Is(err, want) {
			t.Fatalf("write error = %v", err)
		}
		if _, err := e.Finish(transfer.Result{Published: true}, nil); !errors.Is(err, want) || calls != 1 {
			t.Fatalf("wrote after stream failure: calls=%d, error=%v", calls, err)
		}
	}
}

func TestJSONCanceledProgressAllowsTerminalResult(t *testing.T) {
	var out bytes.Buffer
	e, err := newEvents(&out, false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.Update(ctx, transfer.Snapshot{Total: 10}); err != nil || out.Len() != 0 {
		t.Fatalf("canceled progress = %q, %v", &out, err)
	}
	code, err := e.Finish(transfer.Result{}, ctx.Err())
	if err != nil || code != 130 || !strings.Contains(out.String(), `"status":"interrupted"`) {
		t.Fatalf("interrupted terminal result = %q, code=%d, error=%v", &out, code, err)
	}
}

type writeFunc func([]byte) (int, error)

func (f writeFunc) Write(p []byte) (int, error) {
	return f(p)
}
