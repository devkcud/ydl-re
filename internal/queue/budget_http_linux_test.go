package queue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/transfer"
)

func TestQueueSharedBudgetSurvivesLifecycleAndConcurrency(t *testing.T) {
	const data = "abcdefghijklmnopqrstuvwxyz1234"
	var resumes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := data[:20]
		if r.URL.Path == "/pause" {
			body = data
		}
		w.Header().Set("ETag", `"budget"`)
		if r.Header.Get("Range") != "" {
			if r.Header.Get("Range") != "bytes=10-" || r.Header.Get("If-Range") != `"budget"` {
				t.Error("shared-budget resume lost its conditional prefix")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			resumes.Add(1)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)-10))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 10-%d/%d", len(body)-1, len(body)))
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, body[10:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		switch r.URL.Path {
		case "/pause", "/cancel":
			fmt.Fprint(w, body[:10])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/fail":
			fmt.Fprint(w, body[:10])
		default:
			fmt.Fprint(w, body)
		}
	}))
	t.Cleanup(server.Close)
	dir := privateDir(t)
	store := testStore(t, dir)
	start := time.Now()
	budget, err := transfer.NewBudget(100)
	must(t, err)
	var delivered atomic.Int64
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		var last int64
		return budget.Download(ctx, req, func(s transfer.Snapshot) {
			if n := s.Bytes - max(last, s.Resumed); n > 0 {
				total := delivered.Add(n)
				allowed := 10 + 100*(time.Since(start)+5*time.Millisecond).Seconds()
				if float64(total) > allowed {
					t.Errorf("job/control change renewed shared credit: bytes=%d, allowance=%f", total, allowed)
				}
			}
			last = s.Bytes
			progress(s)
		})
	}
	c, err := NewCoordinator(store, run, 1)
	must(t, err)
	done := runCoordinator(t, c)
	add := func(n int, name string) string {
		id := fmt.Sprintf("%032x", n)
		_, err := c.Add(testContext(t), id, transfer.Request{
			URL: server.URL + "/" + name, Output: filepath.Join(dir, name), Connections: 1, Limit: 1000,
		})
		must(t, err)
		return id
	}
	pause := add(1, "pause")
	waitBudgetJob(t, c, pause, func(j JobView) bool { return j.Bytes != nil && *j.Bytes == 10 })
	_, err = c.Pause(testContext(t), pause)
	must(t, err)
	waitBudgetJob(t, c, pause, func(j JobView) bool { return j.State == Paused })
	_, err = c.SetParallel(testContext(t), 3)
	must(t, err)
	cancel := add(2, "cancel")
	fail := add(3, "fail")
	complete := add(4, "complete")
	_, err = c.Resume(testContext(t), pause)
	must(t, err)
	_, err = c.SetParallel(testContext(t), 1)
	must(t, err)
	waitBudgetJob(t, c, cancel, func(j JobView) bool { return j.Bytes != nil && *j.Bytes == 10 })
	_, err = c.Cancel(testContext(t), cancel)
	must(t, err)
	waitBudgetJob(t, c, cancel, func(j JobView) bool { return j.State == Canceled })
	waitBudgetJob(t, c, fail, func(j JobView) bool { return j.State == Failed })
	_, err = c.Resume(testContext(t), fail)
	must(t, err)
	for _, id := range []string{pause, fail, complete} {
		waitBudgetJob(t, c, id, func(j JobView) bool { return j.State == Completed })
	}
	last := add(5, "last")
	waitBudgetJob(t, c, last, func(j JobView) bool { return j.State == Completed })
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
	if delivered.Load() != 100 || resumes.Load() != 2 {
		t.Fatalf("received=%d, resumed=%d; want 100, 2", delivered.Load(), resumes.Load())
	}
	for _, name := range []string{"pause", "fail", "complete", "last", "cancel.yoinkdl.part"} {
		want := data[:20]
		if name == "pause" {
			want = data
		} else if name == "cancel.yoinkdl.part" {
			want = data[:10]
		}
		if got := string(readFile(t, filepath.Join(dir, name))); got != want {
			t.Fatalf("%s contains %q; want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "cancel")); !os.IsNotExist(err) {
		t.Fatal("canceled transfer published an output")
	}
}

func waitBudgetJob(t *testing.T, c *Coordinator, id string, ready func(JobView) bool) {
	t.Helper()
	waitQueue(t, c, func(v ListView) bool {
		for _, j := range v.Jobs {
			if j.ID == id {
				return ready(j)
			}
		}
		return false
	})
}
