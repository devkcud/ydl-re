package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
)

func TestQueueCLIRealTransferAndControls(t *testing.T) {
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
			t.Error("CLI resume lost conditional prefix validation")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", "bytes 5-19/20")
		w.Header().Set("Content-Length", "15")
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, data[5:])
	}))
	t.Cleanup(server.Close)
	paths := queueEnv(t, false)
	owner := startQueueCLI(t, "--parallel", "1", "--limit", "100B/s")
	dir := t.TempDir()
	t.Chdir(dir)
	id := strings.TrimSpace(queueRun(t, "add", server.URL+"/file?token=private-token", "-o", "out\nfile", "-c", "1", "--limit", "1KB/s"))
	if !jobID(id) {
		t.Fatalf("add did not print a job ID: %q", id)
	}
	waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 5 })
	if text := queueRun(t, "list"); strings.Contains(text, "private-token") || !strings.Contains(text, `out\nfile`) {
		t.Fatalf("unsafe list output: %q", text)
	}
	queueRun(t, "pause", id)
	waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].State == queue.Paused })
	part, err := os.ReadFile(filepath.Join(dir, "out\nfile.yoinkdl.part"))
	if err != nil || string(part) != data[:5] {
		t.Fatalf("paused bytes = %q, %v", part, err)
	}
	queueRun(t, "daemon", "set", "--parallel", "2", "--json")
	queueRun(t, "resume", id, "--json")
	view := waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].State == queue.Completed })
	if view.Daemon.Parallel != 2 || view.Daemon.LimitBPS != 100 || view.Jobs[0].Output != filepath.Join(dir, "out\nfile") {
		t.Fatal("CLI lost queue policy or caller-relative output resolution")
	}
	final, err := os.ReadFile(view.Jobs[0].Output)
	if err != nil || string(final) != data || calls.Load() != 2 {
		t.Fatalf("final bytes = %q, %v, calls=%d", final, err, calls.Load())
	}
	stored, err := os.ReadFile(filepath.Join(paths.State, queue.SnapshotName))
	if err != nil || bytes.Contains(stored, []byte("private-token")) {
		t.Fatal("completed request retained its source token")
	}
	queueRun(t, "forget", id, "--json")
	if text := queueRun(t, "list"); !strings.Contains(text, "No jobs.") {
		t.Fatalf("forgotten job retained: %q", text)
	}
	queueRun(t, "daemon", "stop", "--json")
	if code := waitQueueOwner(t, owner); code != 0 || owner.err.Len() != 0 {
		t.Fatalf("daemon stop: code=%d, stderr=%q", code, &owner.err)
	}
}

func TestQueueCLIDefaultNameCancellationAndNoOverwrite(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Length", "10")
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, "abc")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	queueEnv(t, true)
	owner := startQueueCLI(t)
	if !strings.Contains(owner.out.String(), "persistent fallback") {
		t.Fatal("startup did not report runtime fallback")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	record := queueJSON(t, queueRun(t, "add", server.URL+"/named.bin?token=private-token", "-c", "1", "--json"))
	if record.ID == nil || !jobID(*record.ID) {
		t.Fatal("JSON add lost the generated ID")
	}
	waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 3 })
	queueRun(t, "cancel", *record.ID, "--json")
	view := waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].State == queue.Canceled })
	if view.Jobs[0].Output != filepath.Join(dir, "named.bin") {
		t.Fatal("default name did not resolve in the client directory")
	}
	queueRun(t, "forget", *record.ID)
	if part, err := os.ReadFile(filepath.Join(dir, "named.bin.yoinkdl.part")); err != nil || string(part) != "abc" {
		t.Fatal("cancel/forget deleted partial data")
	}
	path := filepath.Join(dir, "existing")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	queueRun(t, "add", server.URL, "-o", path)
	waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].State == queue.Failed })
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" || calls.Load() != 1 {
		t.Fatal("queued retry overwrote a completed output or requested its body")
	}
	queueRun(t, "daemon", "stop")
	if code := waitQueueOwner(t, owner); code != 0 {
		t.Fatalf("daemon exited %d", code)
	}
}

func TestQueueCLINeverStartsDaemonImplicitly(t *testing.T) {
	paths := queueEnv(t, false)
	for _, args := range [][]string{{"list"}, {"daemon", "stop"}, {"daemon", "set", "--parallel", "2"},
		{"add", "https://example.org/?token=private-token", "--json"}, {"pause", strings.Repeat("a", 32)}} {
		var out, errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut, "test")
		if code != 1 || strings.Contains(fmt.Sprint(&out, &errOut), "private-token") {
			t.Fatalf("missing daemon: code=%d, out=%q, err=%q", code, &out, &errOut)
		}
		for _, path := range []string{paths.State, paths.Runtime} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("client created daemon artifacts")
			}
		}
	}
}

type queueOwner struct {
	out    readyOutput
	err    bytes.Buffer
	done   chan int
	cancel context.CancelFunc
}

type readyOutput struct {
	bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func (w *readyOutput) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)
	w.once.Do(func() { close(w.ready) })
	return n, err
}

func queueEnv(t *testing.T, fallback bool) daemon.Paths {
	t.Helper()
	dir, err := os.MkdirTemp("", "ydl-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	runtime := filepath.Join(dir, "run")
	if err := os.Mkdir(runtime, 0o700); err != nil {
		t.Fatal(err)
	}
	if fallback {
		runtime = ""
	}
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	paths, err := daemon.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func startQueueCLI(t *testing.T, args ...string) *queueOwner {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	o := &queueOwner{out: readyOutput{ready: make(chan struct{})}, done: make(chan int, 1), cancel: cancel}
	go func() {
		o.done <- Run(ctx, append([]string{"daemon", "start"}, args...), &o.out, &o.err, "test")
		close(o.done)
	}()
	t.Cleanup(func() {
		cancel()
		waitQueueOwner(t, o)
	})
	select {
	case <-o.out.ready:
	case code := <-o.done:
		t.Fatalf("daemon startup exited %d: %s", code, &o.err)
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not announce readiness")
	}
	return o
}

func waitQueueOwner(t *testing.T, o *queueOwner) int {
	t.Helper()
	select {
	case code := <-o.done:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not finish cleanup")
		return -1
	}
}

func queueRun(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if code := Run(ctx, args, &out, &errOut, "test"); code != 0 || errOut.Len() != 0 {
		t.Fatalf("command %s: code=%d, stdout=%q, stderr=%q", args[0], code, &out, &errOut)
	}
	return out.String()
}

func queueJSON(t *testing.T, text string) queueRecord {
	t.Helper()
	var record queueRecord
	if err := json.Unmarshal([]byte(text), &record); err != nil || record.Version != 1 || record.ExitCode != 0 || record.Error != nil {
		t.Fatalf("JSON result = %+v, %v", record, err)
	}
	return record
}

func waitQueueCLI(t *testing.T, ready func(daemon.List) bool) daemon.List {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		record := queueJSON(t, queueRun(t, "list", "--json"))
		var view daemon.List
		if err := json.Unmarshal(record.Result, &view); err != nil {
			t.Fatal(err)
		}
		if ready(view) {
			return view
		}
	}
	t.Fatal("queue did not reach the expected state")
	return daemon.List{}
}
