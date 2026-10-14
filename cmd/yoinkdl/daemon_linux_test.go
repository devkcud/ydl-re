package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
)

func TestDaemonProcessQueueLifecycle(t *testing.T) {
	const data = "abcdefghijklmnopqrst"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"process"`)
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "20")
			fmt.Fprint(w, data[:5])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"process"` {
			t.Error("process resume lost conditional identity")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Length", "15")
		w.Header().Set("Content-Range", "bytes 5-19/20")
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, data[5:])
	}))
	t.Cleanup(server.Close)
	paths := daemonProcessEnv(t)
	owner := startDaemonProcess(t, "--parallel", "1", "--limit", "100B/s")
	dir := t.TempDir()
	raw := daemonCommandOutput(t, dir, "add", server.URL+"/file?token=private-token", "-o", "client-file", "-c", "1", "--json")
	var added struct {
		Version int    `json:"schema_version"`
		ID      string `json:"job_id"`
		Code    int    `json:"exit_code"`
	}
	if err := json.Unmarshal(raw, &added); err != nil || added.Version != 1 || len(added.ID) != 32 || added.Code != 0 {
		t.Fatalf("add result = %+v, %v", added, err)
	}
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 5 })
	daemonCommandOutput(t, dir, "pause", added.ID)
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].State == queue.Paused })
	daemonCommandOutput(t, dir, "daemon", "set", "--parallel", "2")
	daemonCommandOutput(t, dir, "daemon", "stop", "--json")
	waitJSONExit(t, owner.done, 0)
	if owner.err.Len() != 0 {
		t.Fatalf("owner stderr = %q", &owner.err)
	}
	owner = startDaemonProcess(t)
	view := waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].State == queue.Paused })
	if view.Daemon.Parallel != 2 || view.Daemon.LimitBPS != 0 || view.Jobs[0].Output != filepath.Join(dir, "client-file") {
		t.Fatal("restart lost concurrency, reset rules, or caller-relative output")
	}
	if raw := daemonCommandOutput(t, dir, "list", "--json"); bytes.Contains(raw, []byte("private-token")) {
		t.Fatal("list exposed a private source")
	}
	daemonCommandOutput(t, dir, "resume", added.ID, "--json")
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].State == queue.Completed })
	file := filepath.Join(dir, "client-file")
	if got, err := os.ReadFile(file); err != nil || string(got) != data || calls.Load() != 2 {
		t.Fatalf("published data = %q, %v, requests=%d", got, err, calls.Load())
	}
	daemonCommandOutput(t, dir, "forget", added.ID)
	if _, err := os.Stat(file); err != nil {
		t.Fatal("forget deleted the verified output")
	}
	daemonCommandOutput(t, dir, "daemon", "stop")
	waitJSONExit(t, owner.done, 0)
}

func TestDaemonProcessSignalsJoinAndCheckpoint(t *testing.T) {
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "10")
				w.Header().Set("ETag", `"v1"`)
				fmt.Fprint(w, "abc")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			t.Cleanup(server.Close)
			paths := daemonProcessEnv(t)
			owner := startDaemonProcess(t)
			dir := t.TempDir()
			daemonCommandOutput(t, dir, "add", server.URL+"/file?token=private-token", "-o", "file", "-c", "1")
			waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 3 })
			if err := owner.cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			waitJSONExit(t, owner.done, 130)
			if strings.Contains(owner.err.String(), "private-token") {
				t.Fatal("signal diagnostic exposed a source")
			}
			if got, err := os.ReadFile(filepath.Join(dir, "file.yoinkdl.part")); err != nil || string(got) != "abc" {
				t.Fatal("daemon signal lost the checkpoint")
			}
			store, err := queue.Open(paths.State)
			if err != nil {
				t.Fatal("daemon exited before releasing store ownership", err)
			}
			state, err := store.Snapshot()
			store.Close()
			if err != nil || state.Jobs[0].State != queue.Paused {
				t.Fatal("daemon signal skipped durable result cleanup")
			}
			if _, err := os.Stat(paths.Socket()); !os.IsNotExist(err) {
				t.Fatal("daemon signal retained its endpoint")
			}
		})
	}
}

func TestQueueProcessBrokenAndBlockedOutputKeepsWork(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "10")
				fmt.Fprint(w, "abc")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			t.Cleanup(server.Close)
			paths := daemonProcessEnv(t)
			owner := startDaemonProcess(t)
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			if blocked {
				fillJSONPipe(t, writer)
			} else {
				reader.Close()
			}
			cmd := command("add", server.URL+"?token=private-token", "-o", filepath.Join(t.TempDir(), "file"), "-c", "1", "--json")
			var errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = writer, &errOut
			done := startJSONProcess(t, cmd)
			writer.Close()
			view := waitDaemonJobs(t, paths, func(v daemon.List) bool { return len(v.Jobs) == 1 && v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 3 })
			code := 1
			if blocked {
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
				code = 130
			}
			waitJSONExit(t, done, code)
			if !strings.Contains(errOut.String(), "Job ID: "+view.Jobs[0].ID) || strings.Contains(errOut.String(), "private-token") {
				t.Fatalf("output failure hid the receipt or leaked a source: %q", &errOut)
			}
			waitDaemonJobs(t, paths, func(v daemon.List) bool {
				if v.Jobs[0].State != queue.Running {
					t.Fatal("client output failure canceled daemon work")
				}
				return true
			})
			daemonCommandOutput(t, "", "cancel", view.Jobs[0].ID)
			waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].State == queue.Canceled })
			daemonCommandOutput(t, "", "daemon", "stop")
			waitJSONExit(t, owner.done, 0)
		})
	}
}

func TestDaemonProcessBrokenStartupOutputReleasesOwnership(t *testing.T) {
	paths := daemonProcessEnv(t)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	defer writer.Close()
	cmd := command("daemon", "start")
	var errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = writer, &errOut
	done := startJSONProcess(t, cmd)
	writer.Close()
	waitJSONExit(t, done, 1)
	if !strings.Contains(errOut.String(), "stopped before admitting jobs") {
		t.Fatalf("startup output failure = %q", &errOut)
	}
	if _, err := os.Stat(paths.Socket()); !os.IsNotExist(err) {
		t.Fatal("broken startup output left a socket")
	}
	store, err := queue.Open(paths.State)
	if err != nil {
		t.Fatal("broken startup output retained queue ownership", err)
	}
	store.Close()
}

type daemonProcess struct {
	cmd  *exec.Cmd
	err  bytes.Buffer
	done <-chan error
}

func daemonProcessEnv(t *testing.T) daemon.Paths {
	t.Helper()
	dir, err := os.MkdirTemp("", "ydl-proc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_RUNTIME_DIR", "")
	paths, err := daemon.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func startDaemonProcess(t *testing.T, args ...string) *daemonProcess {
	t.Helper()
	p := &daemonProcess{cmd: command(append([]string{"daemon", "start"}, args...)...)}
	out, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.cmd.Stderr = &p.err
	p.done = startJSONProcess(t, p.cmd)
	ready := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(out)
		var text strings.Builder
		for range 3 {
			line, err := reader.ReadString('\n')
			text.WriteString(line)
			if err != nil {
				break
			}
		}
		ready <- text.String()
	}()
	select {
	case text := <-ready:
		if !strings.HasPrefix(text, "Daemon listening:") || !strings.Contains(text, "persistent fallback") {
			t.Fatalf("startup notice = %q", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not announce readiness")
	}
	return p
}

func daemonCommandOutput(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := command(args...)
	cmd.Dir = dir
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	data, err := cmd.Output()
	if err != nil || errOut.Len() != 0 || bytes.Contains(data, []byte("private-token")) {
		t.Fatalf("%s process = %q, %v, stderr=%q", args[0], data, err, &errOut)
	}
	return data
}

func waitDaemonJobs(t *testing.T, paths daemon.Paths, ready func(daemon.List) bool) daemon.List {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		r, err := daemon.Call(ctx, paths, "list", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		var view daemon.List
		if err := json.Unmarshal(r.Result, &view); err != nil {
			t.Fatal(err)
		}
		if ready(view) {
			return view
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatal("queue process did not reach the expected state")
		}
	}
}
