package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

func TestDaemonDispatchAndSafeViews(t *testing.T) {
	p := testPaths(t)
	started := make(chan transfer.Request, 4)
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		started <- req
		progress(transfer.Snapshot{Bytes: 1, Total: 10, Workers: 1})
		<-ctx.Done()
		return transfer.Result{}, ctx.Err()
	}
	s, done := startServer(t, Options{Paths: p, Limit: 123}, run)
	var status Status
	check(t, json.Unmarshal(call(t, p, "ping", map[string]any{}).Result, &status))
	if status.LimitBPS != 123 || status.Parallel != 1 || status.Active != 0 {
		t.Fatalf("status = %+v", status)
	}
	req := testRequest(t, p)
	args := map[string]any{"id": testID, "request": req}
	first := call(t, p, "add", args)
	select {
	case got := <-started:
		if got.URL != *req.URL || got.Output != req.Output || got.Overwrite {
			t.Fatal("IPC changed normalized engine options")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("accepted job did not start")
	}
	call(t, p, "pause", map[string]any{"id": testID})
	waitList(t, p, func(v List) bool { return v.Jobs[0].State == queue.Paused })
	call(t, p, "resume", map[string]any{"id": testID})
	<-started
	call(t, p, "set_parallel", map[string]any{"parallel": 3})
	view := waitList(t, p, func(v List) bool { return v.Daemon.Parallel == 3 })
	if view.Daemon.LimitBPS != 123 {
		t.Fatal("concurrency change replaced the shared rate")
	}
	call(t, p, "cancel", map[string]any{"id": testID})
	waitList(t, p, func(v List) bool { return v.Jobs[0].State == queue.Canceled })
	retained := call(t, p, "add", args)
	for _, data := range []json.RawMessage{first.Result, retained.Result, call(t, p, "list", map[string]any{}).Result} {
		for _, secret := range []string{"secret", "https://", "input_hash", `"request"`, `"url"`} {
			if strings.Contains(string(data), secret) {
				t.Fatal("private source reached an IPC view")
			}
		}
	}
	call(t, p, "forget", map[string]any{"id": testID})
	waitList(t, p, func(v List) bool { return len(v.Jobs) == 0 })
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
	check(t, s.Close())
	check(t, s.Close())
	if err := s.Run(context.Background()); !errors.Is(err, ErrStarted) {
		t.Fatal("daemon ran twice")
	}
	if _, err := os.Lstat(p.Socket()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed shutdown retained the socket")
	}
}

func TestDaemonStopAcknowledgesBeforeJoining(t *testing.T) {
	p := testPaths(t)
	started := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return transfer.Result{}, ctx.Err()
	}
	s, done := startServer(t, Options{Paths: p}, run)
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	call(t, p, "add", map[string]any{"id": testID, "request": testRequest(t, p)})
	<-started
	r := call(t, p, "stop", map[string]any{})
	var status Status
	check(t, json.Unmarshal(r.Result, &status))
	if status.State != "stopping" || status.Active != 1 {
		t.Fatal("stop claimed completed cleanup")
	}
	<-canceled
	call(t, p, "ping", map[string]any{})
	call(t, p, "stop", map[string]any{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := Call(ctx, p, "set_parallel", map[string]any{"parallel": 2})
	if err == nil || r.Error == nil || r.Error.Code != "stopping" {
		t.Fatal("stopping daemon accepted a mutation")
	}
	if other, err := acquire(p); other != nil || !errors.Is(err, queue.ErrLocked) {
		t.Fatal("shutdown released ownership before joining")
	}
	select {
	case <-s.Done():
		t.Fatal("shutdown finished before engine cleanup")
	default:
	}
	once.Do(func() { close(release) })
	check(t, waitServer(t, done))
	store, err := queue.Open(p.State)
	check(t, err)
	defer store.Close()
	state, err := store.Snapshot()
	check(t, err)
	if state.Jobs[0].State != queue.Paused {
		t.Fatal("shutdown lost the durable paused result")
	}
}

func TestDaemonDisconnectedAddAndPipelining(t *testing.T) {
	p := testPaths(t)
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		<-ctx.Done()
		return transfer.Result{}, ctx.Err()
	}
	_, done := startServer(t, Options{Paths: p}, run)
	args := map[string]any{"id": testID, "request": testRequest(t, p)}
	conn := rawClient(t, p)
	data := append(requestFrame(t, p.State, "ping", map[string]any{}), requestFrame(t, p.State, "add", args)...)
	check(t, writeFrame(conn, data))
	data, err := readFrame(conn, MaxReplySize)
	check(t, err)
	if _, err := decodeReply(data, "ping"); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	waitList(t, p, func(v List) bool {
		if len(v.Jobs) != 0 {
			t.Fatal("pipelined mutation was executed")
		}
		return true
	})
	conn = rawClient(t, p)
	check(t, writeFrame(conn, requestFrame(t, p.State, "add", args)))
	conn.Close()
	view := waitList(t, p, func(v List) bool { return len(v.Jobs) == 1 })
	call(t, p, "add", args)
	again := waitList(t, p, func(v List) bool { return len(v.Jobs) == 1 })
	if view.Jobs[0].Order != again.Jobs[0].Order {
		t.Fatal("retry after lost acknowledgment duplicated the add")
	}
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
}

func TestDaemonStartupFailureRetainsCommittedRecovery(t *testing.T) {
	p := testPaths(t)
	store, err := queue.Open(p.State)
	check(t, err)
	state := queue.NewSnapshot()
	j, err := queue.NewJob(testID, 1, transfer.Request{URL: "https://example.org/?secret=private", Output: filepath.Join(p.State, "out")})
	check(t, err)
	j.State, j.Published = queue.Running, nil
	state.Jobs, state.NextOrder = []queue.Job{j}, 2
	check(t, store.Save(state))
	check(t, store.Close())
	check(t, os.Mkdir(p.Runtime, 0o700))
	check(t, os.WriteFile(p.Socket(), []byte("foreign"), 0o600))
	if s, err := New(Options{Paths: p, Parallel: 3}); s != nil || !errors.Is(err, ErrEndpoint) {
		t.Fatalf("startup = %v, %v", s, err)
	}
	store, err = queue.Open(p.State)
	check(t, err)
	defer store.Close()
	state, err = store.Snapshot()
	check(t, err)
	if state.Parallel != 3 || state.Jobs[0].State != queue.Interrupted || state.Jobs[0].Published != nil {
		t.Fatal("bind failure rolled back committed recovery/settings")
	}
	data, err := os.ReadFile(p.Socket())
	check(t, err)
	if string(data) != "foreign" {
		t.Fatal("failed startup changed the existing endpoint")
	}
}

func TestDaemonCloseBeforeRun(t *testing.T) {
	p := testPaths(t)
	s, err := New(Options{Paths: p})
	check(t, err)
	check(t, s.Close())
	check(t, s.Close())
	if err := s.Run(context.Background()); !errors.Is(err, ErrStarted) {
		t.Fatal("closed daemon ran")
	}
}

func TestDaemonConcurrentCloseJoinsActiveWork(t *testing.T) {
	p := testPaths(t)
	started := make(chan struct{})
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		close(started)
		<-ctx.Done()
		return transfer.Result{}, ctx.Err()
	}
	s, done := startServer(t, Options{Paths: p}, run)
	call(t, p, "add", map[string]any{"id": testID, "request": testRequest(t, p)})
	<-started
	var group sync.WaitGroup
	for range MaxClients * 2 {
		group.Go(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	check(t, waitServer(t, done))
	store, err := queue.Open(p.State)
	check(t, err)
	defer store.Close()
	state, err := store.Snapshot()
	check(t, err)
	if state.Jobs[0].State != queue.Paused {
		t.Fatal("Close lost the durable paused outcome")
	}
}

func TestDaemonContextCancellationPreservesInterruptedExit(t *testing.T) {
	p := testPaths(t)
	started := make(chan struct{})
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		close(started)
		<-ctx.Done()
		return transfer.Result{}, ctx.Err()
	}
	s, err := newServer(Options{Paths: p}, run)
	check(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() { cancel(); s.Close() })
	call(t, p, "add", map[string]any{"id": testID, "request": testRequest(t, p)})
	<-started
	cancel()
	if err := waitServer(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("interrupted daemon exit = %v", err)
	}
	store, err := queue.Open(p.State)
	check(t, err)
	defer store.Close()
	state, err := store.Snapshot()
	check(t, err)
	if state.Jobs[0].State != queue.Paused {
		t.Fatal("context shutdown skipped durable cleanup")
	}
}

func startServer(t *testing.T, opts Options, run queue.Runner) (*Server, <-chan error) {
	t.Helper()
	s, err := newServer(opts, run)
	check(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-s.Done():
		case <-time.After(10 * time.Second):
			t.Error("daemon leaked after cleanup")
		}
	})
	return s, done
}

func call(t *testing.T, p Paths, op string, args any) Reply {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Call(ctx, p, op, args)
	check(t, err)
	return r
}

func waitList(t *testing.T, p Paths, ready func(List) bool) List {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		r, err := Call(ctx, p, "list", map[string]any{})
		check(t, err)
		var view List
		check(t, json.Unmarshal(r.Result, &view))
		if ready(view) {
			return view
		}
	}
}

func waitServer(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit")
		return nil
	}
}

func rawClient(t *testing.T, p Paths) *net.UnixConn {
	t.Helper()
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: p.Socket(), Net: "unix"})
	check(t, err)
	t.Cleanup(func() { conn.Close() })
	check(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	return conn
}

func testRequest(t *testing.T, p Paths) queue.Request {
	t.Helper()
	j, err := queue.NewJob(testID, 1, transfer.Request{URL: "https://example.org/?secret=private", Output: filepath.Join(p.State, "file"), Connections: 1})
	check(t, err)
	return j.Request
}
