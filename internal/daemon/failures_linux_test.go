package daemon

import (
	"context"
	"errors"
	"fmt"
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

func TestDaemonPersistenceFailureStopsWithoutStaleSave(t *testing.T) {
	p := testPaths(t)
	_, done := startServer(t, Options{Paths: p}, nil)
	path := filepath.Join(p.State, queue.SnapshotName)
	const changed = "unrecognized private queue contents"
	check(t, os.WriteFile(path, []byte(changed), 0o600))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := Call(ctx, p, "add", map[string]any{"id": testID, "request": testRequest(t, p)})
	if err == nil || r.Error == nil || r.Error.Code != "persistence" {
		t.Fatalf("persistence reply = %+v, %v", r, err)
	}
	if err := waitServer(t, done); !errors.Is(err, queue.ErrPersistence) {
		t.Fatalf("persistence outcome = %v", err)
	}
	data, err := os.ReadFile(path)
	check(t, err)
	if string(data) != changed {
		t.Fatal("daemon saved stale memory over changed state")
	}
}

func TestDaemonOwnershipLossStopsAndJoins(t *testing.T) {
	p := testPaths(t)
	started := make(chan struct{})
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		close(started)
		<-ctx.Done()
		return transfer.Result{}, ctx.Err()
	}
	_, done := startServer(t, Options{Paths: p}, run)
	call(t, p, "add", map[string]any{"id": testID, "request": testRequest(t, p)})
	<-started
	lock := filepath.Join(p.Runtime, LockName)
	check(t, os.Rename(lock, lock+".saved"))
	check(t, os.WriteFile(lock, []byte("foreign lock"), 0o600))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Call(ctx, p, "ping", map[string]any{}); err == nil {
		t.Fatal("changed runtime ownership remained authoritative")
	}
	if err := waitServer(t, done); !errors.Is(err, ErrChanged) {
		t.Fatalf("ownership loss = %v", err)
	}
	data, err := os.ReadFile(lock)
	check(t, err)
	if string(data) != "foreign lock" {
		t.Fatal("shutdown changed the replacement lock")
	}
	store, err := queue.Open(p.State)
	check(t, err)
	defer store.Close()
	state, err := store.Snapshot()
	check(t, err)
	if state.Jobs[0].State != queue.Paused {
		t.Fatal("ownership loss did not join and save the paused transfer")
	}
}

func TestDaemonBoundedClientsDoNotBlockQueue(t *testing.T) {
	p := testPaths(t)
	finish := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	run := func(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
		close(started)
		select {
		case <-finish:
			return transfer.Result{Published: true}, nil
		case <-ctx.Done():
			return transfer.Result{}, ctx.Err()
		}
	}
	s, done := startServer(t, Options{Paths: p}, run)
	t.Cleanup(func() { once.Do(func() { close(finish) }) })
	call(t, p, "add", map[string]any{"id": testID, "request": testRequest(t, p)})
	<-started
	clients := make([]*net.UnixConn, MaxClients)
	for i := range clients {
		clients[i] = rawClient(t, p)
		check(t, writeFrame(clients[i], []byte("{")))
	}
	last := rawClient(t, p)
	check(t, writeFrame(last, requestFrame(t, p.State, "ping", map[string]any{})))
	check(t, last.SetReadDeadline(time.Now().Add(30*time.Millisecond)))
	if _, err := last.Read(make([]byte, 1)); err == nil {
		t.Fatal("server exceeded its accepted-client bound")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("overflow client = %v", err)
	}
	once.Do(func() { close(finish) })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		view, err := s.queue.List(ctx)
		check(t, err)
		if view.Jobs[0].State == queue.Completed {
			break
		}
	}
	clients[0].Close()
	check(t, last.SetReadDeadline(time.Now().Add(time.Second)))
	data, err := readFrame(last, MaxReplySize)
	check(t, err)
	if _, err := decodeReply(data, "ping"); err != nil {
		t.Fatal(err)
	}
	for _, conn := range clients {
		conn.Close()
	}
	last.Close()
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
}

func TestDaemonReadDeadlineReleasesClients(t *testing.T) {
	p := testPaths(t)
	_, done := startServer(t, Options{Paths: p}, nil)
	conn := rawClient(t, p)
	check(t, conn.SetDeadline(time.Now().Add(Timeout+2*time.Second)))
	start := time.Now()
	check(t, writeFrame(conn, []byte("{")))
	data, err := readFrame(conn, MaxReplySize)
	check(t, err)
	r, err := decodeReply(data, "ping")
	if err != nil || r.OK || r.Error.Code != "invalid_request" || time.Since(start) < Timeout-50*time.Millisecond {
		t.Fatalf("read deadline reply = %+v, %v, elapsed=%s", r, err, time.Since(start))
	}
	conn.Close()
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
}

func TestDaemonBlockedReplyDoesNotBlockControlsOrCleanup(t *testing.T) {
	p := testPaths(t)
	store, err := queue.Open(p.State)
	check(t, err)
	state := queue.NewSnapshot()
	for i := range queue.MaxJobs {
		j, err := queue.NewJob(fmt.Sprintf("%032x", i+1), int64(i+1), transfer.Request{
			URL: "https://example.org/?secret=private", Output: "/" + strings.Repeat("x", 4000) + fmt.Sprint(i),
		})
		check(t, err)
		j.State = queue.Paused
		state.Jobs = append(state.Jobs, j)
	}
	state.NextOrder = queue.MaxJobs + 1
	check(t, store.Save(state))
	check(t, store.Close())
	_, done := startServer(t, Options{Paths: p}, nil)
	conn := rawClient(t, p)
	check(t, writeFrame(conn, requestFrame(t, p.State, "list", map[string]any{})))
	if n, err := conn.Read(make([]byte, 1)); n != 1 || err != nil {
		t.Fatalf("large response did not begin: %d, %v", n, err)
	}
	call(t, p, "set_parallel", map[string]any{"parallel": 2})
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
}
