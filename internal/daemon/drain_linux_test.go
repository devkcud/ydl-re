package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

func TestDaemonProcessRetainsDrainingSlot(t *testing.T) {
	for _, op := range []string{"pause", "cancel"} {
		t.Run(op, func(t *testing.T) {
			var calls atomic.Int32
			release := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Length", "10")
				w.Header().Set("ETag", `"draining"`)
				fmt.Fprint(w, "abcde")
				w.(http.Flusher).Flush()
				select {
				case <-release:
					fmt.Fprint(w, "fghij")
				case <-r.Context().Done():
				}
			}))
			t.Cleanup(srv.Close)

			paths := testPaths(t)
			p := startDaemonTestProcess(t, paths, "TestDaemonDrainProcess")
			first := crashJob(t, paths, srv.URL)
			second, err := queue.NewJob(fmt.Sprintf("%032x", 2), 2, transfer.Request{
				URL: srv.URL, Output: filepath.Join(paths.State, "waiting"), Connections: 1,
			})
			check(t, err)
			call(t, paths, "add", map[string]any{"id": first.ID, "request": first.Request})
			waitList(t, paths, func(v List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 5 })
			call(t, paths, "add", map[string]any{"id": second.ID, "request": second.Request})
			call(t, paths, op, map[string]any{"id": first.ID})
			p.event(t, "draining")

			pending, settled := queue.Pausing, queue.Paused
			if op == "cancel" {
				pending, settled = queue.Canceling, queue.Canceled
			}
			view := waitList(t, paths, func(v List) bool { return v.Jobs[0].State == pending })
			if view.Daemon.Active != 1 || view.Jobs[1].State != queue.Queued || view.Jobs[1].Bytes != nil || calls.Load() != 1 {
				t.Fatal("pending result released its whole-job slot")
			}
			call(t, paths, "set_parallel", map[string]any{"parallel": 1})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			r, err := Call(ctx, paths, "forget", map[string]any{"id": first.ID})
			cancel()
			if err == nil || r.Error == nil || r.Error.Code != "busy" {
				t.Fatal("draining job was forgotten before its runner returned")
			}
			checkCrashFile(t, first.Request.Output+storage.PartSuffix, []byte("abcde"))

			_, err = p.input.Write([]byte{1})
			check(t, err)
			view = waitList(t, paths, func(v List) bool { return v.Jobs[0].State == settled && v.Jobs[1].Bytes != nil })
			if view.Daemon.Active != 1 || calls.Load() != 2 {
				t.Fatal("returned runner did not release exactly one slot")
			}
			close(release)
			waitList(t, paths, func(v List) bool { return v.Jobs[1].State == queue.Completed })
			call(t, paths, "stop", map[string]any{})
			p.wait(t, false)
			checkCrashFile(t, first.Request.Output+storage.PartSuffix, []byte("abcde"))
			checkCrashFile(t, second.Request.Output, []byte("abcdefghij"))
		})
	}
}

func TestDaemonDrainProcess(t *testing.T) {
	if os.Getenv("YOINKDL_DAEMON_CRASH_PROCESS") != "1" {
		return
	}
	paths := Paths{State: os.Getenv("YOINKDL_DAEMON_CRASH_STATE"), Runtime: os.Getenv("YOINKDL_DAEMON_CRASH_RUNTIME")}
	budget, err := transfer.NewBudget(0)
	check(t, err)
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		result, err := budget.Download(ctx, req, progress)
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stdout, "draining")
			if _, cause := io.ReadFull(os.Stdin, make([]byte, 1)); cause != nil {
				return result, cause
			}
		}
		return result, err
	}
	s, err := newServer(Options{Paths: paths}, run)
	check(t, err)
	fmt.Fprintln(os.Stdout, "ready")
	check(t, s.Run(context.Background()))
}
