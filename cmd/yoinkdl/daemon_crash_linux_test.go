package main

import (
	"bytes"
	"crypto/sha256"
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
)

func TestDaemonProcessCrashPreservesQueueAndResume(t *testing.T) {
	const sent = storage.CheckpointBytes + 64*1024
	type job struct {
		id      string
		data    []byte
		offset  atomic.Int64
		calls   atomic.Int32
		release chan struct{}
	}
	jobs := make([]*job, 5)
	for i := range jobs {
		data := []byte(fmt.Sprintf("queued file %d", i))
		if i < 2 {
			data = bytes.Repeat([]byte{byte('a' + i)}, 2*storage.CheckpointBytes)
		}
		jobs[i] = &job{data: data, release: make(chan struct{})}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil || i < 0 || i >= len(jobs) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		j := jobs[i]
		n := j.calls.Add(1)
		w.Header().Set("ETag", `"crash-version"`)
		if i < 2 && n > 1 {
			start := j.offset.Load()
			if start <= 0 || r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", start) || r.Header.Get("If-Range") != `"crash-version"` {
				t.Error("crashed job did not resume its durable suffix with the saved validator")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Length", strconv.FormatInt(int64(len(j.data))-start, 10))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(j.data)-1, len(j.data)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(j.data[start:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(j.data)))
		if i < 2 {
			w.Write(j.data[:sent])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		w.(http.Flusher).Flush()
		select {
		case <-j.release:
			w.Write(j.data)
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	paths := daemonProcessEnv(t)
	owner := startDaemonProcess(t)
	daemonCommandOutput(t, "", "daemon", "set", "--parallel", "2")
	dir := t.TempDir()
	for i, j := range jobs {
		name := strconv.Itoa(i)
		j.id = strings.TrimSpace(string(daemonCommandOutput(t, dir, "add", server.URL+"/"+name+"?token=private-token", "-o", name, "-c", "1")))
	}
	view := waitDaemonJobs(t, paths, func(v daemon.List) bool {
		return len(v.Jobs) == len(jobs) && v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == sent &&
			v.Jobs[1].Bytes != nil && *v.Jobs[1].Bytes == sent
	})
	for i, j := range view.Jobs {
		want := queue.Queued
		if i < 2 {
			want = queue.Running
		}
		if j.ID != jobs[i].id || j.Order != int64(i+1) || j.State != want {
			t.Fatal("initial queue lost admission order or active bounds")
		}
	}
	locks := []string{filepath.Join(paths.State, queue.LockName), filepath.Join(paths.Runtime, "daemon.lock")}
	var inodes []os.FileInfo
	for _, path := range locks {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		inodes = append(inodes, info)
	}
	if err := owner.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitJSONExit(t, owner.done, -1)
	if owner.err.Len() != 0 {
		t.Fatalf("crashed owner stderr = %q", &owner.err)
	}
	if _, err := os.Lstat(paths.Socket()); err != nil {
		t.Fatal("forced death did not retain the stale endpoint", err)
	}
	state := daemonSnapshot(t, paths)
	if state.Parallel != 2 || state.NextOrder != 6 || len(state.Jobs) != len(jobs) {
		t.Fatal("forced death lost saved settings or jobs")
	}
	for i, j := range state.Jobs {
		if j.ID != view.Jobs[i].ID || j.Order != view.Jobs[i].Order || j.State != view.Jobs[i].State ||
			j.Request.URL == nil || !strings.Contains(*j.Request.URL, "private-token") {
			t.Fatal("forced death changed a committed intent or source")
		}
		if i >= 2 {
			continue
		}
		path := filepath.Join(dir, strconv.Itoa(i))
		m, err := storage.Inspect(path)
		if err != nil || m.Completed <= 0 || m.Completed > sent || m.State != storage.StatePartial {
			t.Fatalf("invalid crash checkpoint: %+v, %v", m, err)
		}
		jobs[i].offset.Store(m.Completed)
		part, err := os.ReadFile(path + storage.PartSuffix)
		if err != nil || !bytes.Equal(part, jobs[i].data[:sent]) {
			t.Fatal("forced death changed the received prefix", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("crash published an incomplete output")
		}
	}
	owner = startDaemonProcess(t)
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool {
		return len(v.Jobs) == len(jobs) && v.Jobs[2].Bytes != nil && v.Jobs[3].Bytes != nil
	})
	if view.Daemon.Parallel != 2 || view.Daemon.Active != 2 {
		t.Fatal("restart lost the configured parallel limit")
	}
	for i, j := range view.Jobs {
		want := queue.Interrupted
		if i == 2 || i == 3 {
			want = queue.Running
		} else if i == 4 {
			want = queue.Queued
		}
		if j.ID != jobs[i].id || j.Order != state.Jobs[i].Order || j.State != want {
			t.Fatal("restart did not reconcile active intents and preserve FIFO admission")
		}
		if i < 2 && (j.Published != nil || j.Bytes != nil || j.Error == nil || jobs[i].calls.Load() != 1) {
			t.Fatal("restart inferred an outcome or replayed an interrupted job")
		}
	}
	for i, path := range locks {
		info, err := os.Stat(path)
		if err != nil || !os.SameFile(inodes[i], info) {
			t.Fatal("restart replaced a persistent ownership lock", err)
		}
	}
	daemonCommandOutput(t, "", "resume", jobs[0].id)
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[4].ID == jobs[0].id })
	if view.Jobs[4].Order != state.NextOrder || view.Jobs[4].State != queue.Queued || jobs[0].calls.Load() != 1 {
		t.Fatal("explicit resume did not join the FIFO tail")
	}
	close(jobs[2].release)
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[3].Bytes != nil })
	if jobs[4].calls.Load() != 1 || jobs[0].calls.Load() != 1 {
		t.Fatal("resumed job bypassed earlier queued work")
	}
	close(jobs[3].release)
	close(jobs[4].release)
	waitDaemonJobs(t, paths, func(v daemon.List) bool {
		for _, j := range v.Jobs {
			if j.ID == jobs[1].id {
				if j.State != queue.Interrupted || jobs[1].calls.Load() != 1 {
					t.Fatal("idle capacity replayed an interrupted job")
				}
			} else if j.State != queue.Completed {
				return false
			}
		}
		return true
	})
	daemonCommandOutput(t, "", "resume", jobs[1].id)
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[4].State == queue.Completed })
	daemonCommandOutput(t, "", "daemon", "stop")
	waitJSONExit(t, owner.done, 0)
	state = daemonSnapshot(t, paths)
	for i, j := range jobs {
		path := filepath.Join(dir, strconv.Itoa(i))
		got, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(got) != sha256.Sum256(j.data) {
			t.Fatal("crash recovery published incorrect bytes", err)
		}
		want := int32(1)
		if i < 2 {
			want = 2
		}
		if j.calls.Load() != want || state.Jobs[i].State != queue.Completed || state.Jobs[i].Request.URL != nil {
			t.Fatal("crash recovery repeated requests or retained a completed source")
		}
		for _, suffix := range []string{storage.PartSuffix, storage.ManifestSuffix} {
			if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
				t.Fatalf("completed job retained %s: %v", suffix, err)
			}
		}
	}
}

func daemonSnapshot(t *testing.T, paths daemon.Paths) queue.Snapshot {
	t.Helper()
	store, err := queue.Open(paths.State)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	state, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
