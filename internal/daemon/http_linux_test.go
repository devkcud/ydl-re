package daemon

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

func TestDaemonHTTPPauseResumeAndSharedLimit(t *testing.T) {
	const data = "abcdefghijklmnopqrst"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			fmt.Fprint(w, data[:5])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"v1"` {
			t.Error("IPC resume lost conditional prefix validation")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Length", "15")
		w.Header().Set("Content-Range", "bytes 5-19/20")
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, data[5:])
	}))
	t.Cleanup(server.Close)
	p := testPaths(t)
	start := time.Now()
	_, done := startServer(t, Options{Paths: p, Limit: 100}, nil)
	j, err := queue.NewJob(testID, 1, transfer.Request{
		URL: server.URL + "/file?secret=private", Output: filepath.Join(p.State, "file"), Connections: 1, Limit: 1000,
	})
	check(t, err)
	call(t, p, "add", map[string]any{"id": testID, "request": j.Request})
	waitList(t, p, func(v List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 5 })
	call(t, p, "pause", map[string]any{"id": testID})
	waitList(t, p, func(v List) bool { return v.Jobs[0].State == queue.Paused })
	part, err := os.ReadFile(j.Request.Output + ".yoinkdl.part")
	check(t, err)
	if string(part) != data[:5] {
		t.Fatal("pause did not preserve its verified checkpoint")
	}
	call(t, p, "resume", map[string]any{"id": testID})
	view := waitList(t, p, func(v List) bool { return v.Jobs[0].State == queue.Completed })
	if view.Daemon.LimitBPS != 100 || calls.Load() != 2 {
		t.Fatal("daemon lost its shared cap or repeated transfer requests")
	}
	if float64(len(data)) > 10+100*(time.Since(start)+5*time.Millisecond).Seconds() {
		t.Fatal("daemon downloads bypassed the shared budget")
	}
	final, err := os.ReadFile(j.Request.Output)
	check(t, err)
	if string(final) != data {
		t.Fatal("daemon published incorrect bytes")
	}
	stored, err := os.ReadFile(filepath.Join(p.State, queue.SnapshotName))
	check(t, err)
	if bytes.Contains(stored, []byte("private")) {
		t.Fatal("completed IPC job retained the source token")
	}
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
}
