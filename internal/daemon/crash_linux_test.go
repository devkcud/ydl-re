package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"yoinkdl/internal/queue"
	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

func TestDaemonProcessCrashAfterPublication(t *testing.T) {
	for _, op := range []string{"running", "pause", "cancel"} {
		t.Run(op, func(t *testing.T) {
			const data = "verified output before queue completion"
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(w, data)
			}))
			t.Cleanup(srv.Close)
			paths := testPaths(t)
			p := startCrashProcess(t, paths, true)
			j := crashJob(t, paths, srv.URL)
			call(t, paths, "add", map[string]any{"id": j.ID, "request": j.Request})
			p.event(t, "published")
			state, recovered := queue.Running, queue.Interrupted
			switch op {
			case "pause":
				state, recovered = queue.Pausing, queue.Paused
			case "cancel":
				state, recovered = queue.Canceling, queue.Canceled
			}
			if op != "running" {
				call(t, paths, op, map[string]any{"id": j.ID})
			}
			view := waitList(t, paths, func(v List) bool { return v.Jobs[0].State == state })
			if view.Daemon.Active != 1 || view.Jobs[0].Published != nil {
				t.Fatal("held result released its slot or resolved publication")
			}
			file := stat(t, j.Request.Output)
			checkCrashFile(t, j.Request.Output, []byte(data))
			for _, suffix := range []string{storage.PartSuffix, storage.ManifestSuffix} {
				if _, err := os.Stat(j.Request.Output + suffix); !os.IsNotExist(err) {
					t.Fatal("transfer did not finish publication cleanup", err)
				}
			}
			check(t, p.cmd.Process.Kill())
			p.wait(t, true)
			saved := crashSnapshot(t, paths).Jobs[0]
			if saved.State != state || saved.Published != nil || saved.Request.URL == nil {
				t.Fatal("forced death recorded an unreturned transfer result")
			}
			p = startCrashProcess(t, paths, false)
			view = waitList(t, paths, func(v List) bool { return v.Jobs[0].State == recovered })
			if view.Daemon.Active != 0 || view.Jobs[0].Published != nil || view.Jobs[0].Bytes != nil || calls.Load() != 1 {
				t.Fatal("restart inferred success or replayed a published attempt")
			}
			saved = crashSnapshot(t, paths).Jobs[0]
			if saved.State != recovered || saved.Published != nil || (saved.Request.URL == nil) != (recovered == queue.Canceled) {
				t.Fatal("restart did not persist recovery and source retention")
			}
			if raw := call(t, paths, "list", map[string]any{}).Result; bytes.Contains(raw, []byte("secret")) {
				t.Fatal("crash recovery exposed the saved source")
			}
			if recovered == queue.Canceled {
				refuseCrashResume(t, paths, j.ID)
			} else {
				call(t, paths, "resume", map[string]any{"id": j.ID})
				view = waitList(t, paths, func(v List) bool { return v.Jobs[0].State == queue.Failed })
				if view.Jobs[0].Published == nil || *view.Jobs[0].Published || view.Jobs[0].Error == nil {
					t.Fatal("retry inferred success from a preexisting final file")
				}
				call(t, paths, "cancel", map[string]any{"id": j.ID})
			}
			if calls.Load() != 1 {
				t.Fatal("recovery fetched an already published output")
			}
			if crashSnapshot(t, paths).Jobs[0].Request.URL != nil {
				t.Fatal("terminal recovery retained its source")
			}
			call(t, paths, "forget", map[string]any{"id": j.ID})
			call(t, paths, "stop", map[string]any{})
			p.wait(t, false)
			if len(crashSnapshot(t, paths).Jobs) != 0 || !os.SameFile(file, stat(t, j.Request.Output)) {
				t.Fatal("recovery, retry, or forget replaced the final output")
			}
			checkCrashFile(t, j.Request.Output, []byte(data))
		})
	}
}

func TestDaemonProcessCrashDuringControlCleanup(t *testing.T) {
	for _, op := range []string{"pause", "cancel"} {
		t.Run(op, func(t *testing.T) {
			const data = "abcdefghijklmnopqrst"
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"control-crash"`)
				if calls.Add(1) == 1 {
					w.Header().Set("Content-Length", "20")
					fmt.Fprint(w, data[:5])
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"control-crash"` {
					t.Error("recovered pause lost its checkpoint or validator")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Length", "15")
				w.Header().Set("Content-Range", "bytes 5-19/20")
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, data[5:])
			}))
			t.Cleanup(srv.Close)
			paths := testPaths(t)
			p := startCrashProcess(t, paths, true)
			j := crashJob(t, paths, srv.URL)
			call(t, paths, "add", map[string]any{"id": j.ID, "request": j.Request})
			waitList(t, paths, func(v List) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 5 })
			call(t, paths, op, map[string]any{"id": j.ID})
			p.event(t, "canceled")
			state, recovered := queue.Pausing, queue.Paused
			if op == "cancel" {
				state, recovered = queue.Canceling, queue.Canceled
			}
			view := waitList(t, paths, func(v List) bool { return v.Jobs[0].State == state })
			if view.Daemon.Active != 1 || view.Jobs[0].Published != nil {
				t.Fatal("pending control released its slot or inferred a result")
			}
			check(t, p.cmd.Process.Kill())
			p.wait(t, true)
			saved := crashSnapshot(t, paths).Jobs[0]
			if saved.State != state || saved.Request.URL == nil || saved.Published != nil {
				t.Fatal("forced death lost the committed control intent")
			}
			m, err := storage.Inspect(j.Request.Output)
			check(t, err)
			if m.Completed != 5 || m.State != storage.StatePartial {
				t.Fatal("control cleanup lost the durable prefix")
			}
			part := j.Request.Output + storage.PartSuffix
			manifest := j.Request.Output + storage.ManifestSuffix
			meta, err := os.ReadFile(manifest)
			check(t, err)
			checkCrashFile(t, part, []byte(data[:5]))
			p = startCrashProcess(t, paths, false)
			view = waitList(t, paths, func(v List) bool { return v.Jobs[0].State == recovered })
			if view.Daemon.Active != 0 || view.Jobs[0].Published != nil || view.Jobs[0].Bytes != nil || calls.Load() != 1 {
				t.Fatal("restart replayed a pending control or inferred an outcome")
			}
			saved = crashSnapshot(t, paths).Jobs[0]
			if saved.State != recovered || saved.Published != nil || (saved.Request.URL == nil) != (op == "cancel") {
				t.Fatal("restart did not persist the control recovery")
			}
			checkCrashFile(t, part, []byte(data[:5]))
			checkCrashFile(t, manifest, meta)
			if _, err := os.Stat(j.Request.Output); !os.IsNotExist(err) {
				t.Fatal("control recovery published incomplete data")
			}
			if op == "pause" {
				call(t, paths, "resume", map[string]any{"id": j.ID})
				waitList(t, paths, func(v List) bool { return v.Jobs[0].State == queue.Completed })
				if calls.Load() != 2 {
					t.Fatal("explicit resume repeated requests")
				}
			} else {
				refuseCrashResume(t, paths, j.ID)
				if calls.Load() != 1 {
					t.Fatal("canceled recovery made another request")
				}
			}
			if crashSnapshot(t, paths).Jobs[0].Request.URL != nil {
				t.Fatal("terminal job retained its source")
			}
			call(t, paths, "forget", map[string]any{"id": j.ID})
			call(t, paths, "stop", map[string]any{})
			p.wait(t, false)
			if len(crashSnapshot(t, paths).Jobs) != 0 {
				t.Fatal("forget did not persist metadata removal")
			}
			if op == "pause" {
				checkCrashFile(t, j.Request.Output, []byte(data))
				for _, path := range []string{part, manifest} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("completed recovery retained a sidecar", err)
					}
				}
			} else {
				checkCrashFile(t, part, []byte(data[:5]))
				checkCrashFile(t, manifest, meta)
			}
		})
	}
}

func TestDaemonCrashProcess(t *testing.T) {
	if os.Getenv("YOINKDL_DAEMON_CRASH_PROCESS") != "1" {
		return
	}
	paths := Paths{State: os.Getenv("YOINKDL_DAEMON_CRASH_STATE"), Runtime: os.Getenv("YOINKDL_DAEMON_CRASH_RUNTIME")}
	var run queue.Runner
	if os.Getenv("YOINKDL_DAEMON_CRASH_HOLD") == "true" {
		budget, err := transfer.NewBudget(0)
		check(t, err)
		run = func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
			result, err := budget.Download(ctx, req, progress)
			switch {
			case err == nil && result.Published:
				fmt.Fprintln(os.Stdout, "published")
			case errors.Is(err, context.Canceled) && !result.Published:
				fmt.Fprintln(os.Stdout, "canceled")
			default:
				t.Errorf("unexpected held transfer result: %+v, %v", result, err)
				return result, err
			}
			select {}
		}
	}
	s, err := newServer(Options{Paths: paths}, run)
	check(t, err)
	fmt.Fprintln(os.Stdout, "ready")
	check(t, s.Run(context.Background()))
}

type crashProcess struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	done   <-chan error
	events <-chan string
	err    bytes.Buffer
}

func startCrashProcess(t *testing.T, paths Paths, hold bool) *crashProcess {
	t.Helper()
	return startDaemonTestProcess(t, paths, "TestDaemonCrashProcess", fmt.Sprintf("YOINKDL_DAEMON_CRASH_HOLD=%t", hold))
}

func startDaemonTestProcess(t *testing.T, paths Paths, name string, env ...string) *crashProcess {
	t.Helper()
	p := &crashProcess{cmd: exec.Command(os.Args[0], "-test.run=^"+name+"$")}
	p.cmd.Env = append(os.Environ(), "YOINKDL_DAEMON_CRASH_PROCESS=1", "YOINKDL_DAEMON_CRASH_STATE="+paths.State,
		"YOINKDL_DAEMON_CRASH_RUNTIME="+paths.Runtime)
	p.cmd.Env = append(p.cmd.Env, env...)
	in, err := p.cmd.StdinPipe()
	check(t, err)
	p.input = in
	t.Cleanup(func() { in.Close() })
	out, err := p.cmd.StdoutPipe()
	check(t, err)
	p.cmd.Stderr = &p.err
	check(t, p.cmd.Start())
	done := make(chan error, 1)
	p.done = done
	go func() {
		done <- p.cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			p.cmd.Process.Kill()
			<-done
		}
	})
	events := make(chan string, 4)
	p.events = events
	go func() {
		defer close(events)
		scan := bufio.NewScanner(out)
		for scan.Scan() {
			events <- scan.Text()
		}
	}()
	p.event(t, "ready")
	return p
}

func (p *crashProcess) event(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-p.events:
		if got != want {
			t.Fatalf("daemon event = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("daemon did not reach %s", want)
	}
}

func (p *crashProcess) wait(t *testing.T, killed bool) {
	t.Helper()
	select {
	case err := <-p.done:
		if killed {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatalf("daemon was not forcibly killed: %v", err)
			}
		} else {
			check(t, err)
		}
		if p.err.Len() != 0 {
			t.Fatalf("daemon stderr = %q", &p.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon process did not exit")
	}
}

func crashJob(t *testing.T, paths Paths, url string) queue.Job {
	t.Helper()
	j, err := queue.NewJob(testID, 1, transfer.Request{
		URL: url + "?secret=private", Output: filepath.Join(paths.State, "file"), Connections: 1,
	})
	check(t, err)
	return j
}

func crashSnapshot(t *testing.T, paths Paths) queue.Snapshot {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(paths.State, queue.SnapshotName))
	check(t, err)
	var state queue.Snapshot
	check(t, json.Unmarshal(data, &state))
	check(t, state.Validate())
	return state
}

func checkCrashFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	check(t, err)
	if !bytes.Equal(got, want) {
		t.Fatalf("crash recovery changed %s", path)
	}
}

func refuseCrashResume(t *testing.T, paths Paths, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Call(ctx, paths, "resume", map[string]any{"id": id})
	if err == nil || r.Error == nil || r.Error.Code != "conflict" || strings.Contains(err.Error(), "secret") {
		t.Fatal("canceled job did not safely refuse resume")
	}
}
