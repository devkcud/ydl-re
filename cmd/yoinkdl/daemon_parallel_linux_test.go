package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

func TestDaemonProcessParallelControls(t *testing.T) {
	const data = "abcdefghijklmnopqrst"
	const count = queue.MaxParallel + 2
	var calls [count]atomic.Int32
	var release [count]chan struct{}
	for i := range release {
		release[i] = make(chan struct{})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil || i < 0 || i >= count {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls[i].Add(1)
		w.Header().Set("Content-Length", "20")
		w.Header().Set("ETag", `"parallel-controls"`)
		fmt.Fprint(w, data[:5])
		w.(http.Flusher).Flush()
		select {
		case <-release[i]:
			fmt.Fprint(w, data[5:])
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)

	paths := daemonProcessEnv(t)
	owner := startDaemonProcess(t)
	dir := t.TempDir()
	jobs := make([]queue.Job, count)
	for i := range jobs {
		name := strconv.Itoa(i)
		j, err := queue.NewJob(fmt.Sprintf("%032x", i+1), 1, transfer.Request{
			URL: srv.URL + "/" + name + "?token=private-token", Output: filepath.Join(dir, name), Connections: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		jobs[i] = j
		daemonProcessCall(t, paths, "add", map[string]any{"id": j.ID, "request": j.Request}, "")
	}

	view := waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 5 })
	if view.Daemon.Parallel != 1 || view.Daemon.Active != 1 {
		t.Fatal("default daemon admission was not sequential")
	}
	for _, j := range view.Jobs[1:] {
		if j.State != queue.Queued || j.Bytes != nil {
			t.Fatal("sequential daemon admitted waiting work")
		}
	}

	daemonProcessCall(t, paths, "set_parallel", map[string]any{"parallel": queue.MaxParallel}, "")
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool {
		for _, j := range v.Jobs[:queue.MaxParallel] {
			if j.Bytes == nil || *j.Bytes != 5 {
				return false
			}
		}
		return true
	})
	if view.Daemon.Parallel != queue.MaxParallel || view.Daemon.Active != queue.MaxParallel {
		t.Fatal("live increase did not admit the configured whole-job limit")
	}
	for i, j := range view.Jobs {
		want := queue.Running
		if i >= queue.MaxParallel {
			want = queue.Queued
		}
		if j.ID != jobs[i].ID || j.Order != int64(i+1) || j.State != want {
			t.Fatal("live increase changed FIFO admission order")
		}
	}

	daemonProcessCall(t, paths, "set_parallel", map[string]any{"parallel": 2}, "")
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Daemon.Parallel == 2 })
	if view.Daemon.Active != queue.MaxParallel {
		t.Fatal("live reduction preempted active transfers")
	}
	for i := 0; i < queue.MaxParallel-2; i++ {
		close(release[i])
		want := queue.MaxParallel - i - 1
		view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Daemon.Active == want })
		if view.Jobs[i].State != queue.Completed || view.Jobs[8].State != queue.Queued || view.Jobs[9].State != queue.Queued {
			t.Fatal("reduced limit admitted work before active transfers drained below it")
		}
	}

	daemonProcessCall(t, paths, "cancel", map[string]any{"id": jobs[6].ID}, "")
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[6].State == queue.Canceled && v.Jobs[8].Bytes != nil })
	if view.Daemon.Active != 2 || view.Jobs[7].State != queue.Running || view.Jobs[9].State != queue.Queued {
		t.Fatal("cancellation did not release exactly one FIFO slot")
	}

	daemonProcessCall(t, paths, "stop", map[string]any{}, "")
	waitJSONExit(t, owner.done, 0)
	if owner.err.Len() != 0 {
		t.Fatalf("daemon stderr = %q", &owner.err)
	}
	state := daemonSnapshot(t, paths)
	if state.Parallel != 2 || state.Jobs[7].State != queue.Paused || state.Jobs[8].State != queue.Paused || state.Jobs[9].State != queue.Queued {
		t.Fatal("stop lost the reduced limit or active/waiting states")
	}

	owner = startDaemonProcess(t)
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[9].Bytes != nil })
	if view.Daemon.Parallel != 2 || view.Daemon.Active != 1 || view.Jobs[7].State != queue.Paused || view.Jobs[8].State != queue.Paused {
		t.Fatal("restart lost the reduced limit or resumed paused jobs implicitly")
	}
	close(release[9])
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[9].State == queue.Completed })
	daemonProcessCall(t, paths, "stop", map[string]any{}, "")
	waitJSONExit(t, owner.done, 0)

	for i, j := range jobs {
		if calls[i].Load() != 1 {
			t.Fatal("parallel controls repeated or skipped a transfer request")
		}
		path, want := j.Request.Output, data
		if i >= 6 && i <= 8 {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("canceled or paused transfer published output")
			}
			path += storage.PartSuffix
			want = data[:5]
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("parallel control output mismatch for job %d: %v", i, err)
		}
	}
}
