package queue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestQueueParallelIncreaseWaitsForCommit(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	must(t, s.Save(testSnapshot(t, dir, 3)))
	entered, release := blockQueueCommit(t, s)
	defer close(release)
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	a := f.next(t)
	listQueue(t, c)
	reply := make(chan error, 1)
	ctx := testContext(t)
	go func() {
		_, err := c.SetParallel(ctx, 3)
		reply <- err
	}()
	waitCommit(t, entered)
	f.assertIdle(t)
	select {
	case err := <-reply:
		t.Fatalf("setting was acknowledged before sync: %v", err)
	default:
	}
	release <- struct{}{}
	must(t, waitRun(t, reply))
	b, d := f.next(t), f.next(t)
	if listQueue(t, c).Daemon.Active != 3 {
		t.Fatal("durable increase did not admit waiting jobs")
	}
	_, err = c.Stop(testContext(t))
	must(t, err)
	for _, a := range []*fakeAttempt{a, b, d} {
		a.exit <- outcome{err: context.Canceled}
	}
	must(t, waitRun(t, done))
}

func TestQueueDisconnectedAddSurvivesWithoutAliasingCaller(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	entered, release := blockQueueCommit(t, s)
	defer close(release)
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	job := testJob(t, dir, 1)
	req, err := job.Request.Transfer()
	must(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reply := make(chan error, 1)
	go func() {
		_, err := c.Add(ctx, job.ID, req)
		reply <- err
	}()
	waitCommit(t, entered)
	f.assertIdle(t)
	cancel()
	if err := waitRun(t, reply); !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatal("disconnected add falsely reported rollback")
	}
	*req.Retries = 99
	release <- struct{}{}
	a := f.next(t)
	if *a.req.Retries != job.Request.Retries || a.req.URL != *job.Request.URL {
		t.Fatal("caller storage changed an accepted request")
	}
	if addJob(t, c, job).State != Running || len(listQueue(t, c).Jobs) != 1 {
		t.Fatal("retrying an uncertain add duplicated work")
	}
	f.assertIdle(t)
	_, err = c.Stop(testContext(t))
	must(t, err)
	a.exit <- outcome{err: context.Canceled}
	must(t, waitRun(t, done))
}

func blockQueueCommit(t *testing.T, s *Store) (<-chan struct{}, chan struct{}) {
	t.Helper()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var blocked atomic.Bool
	s.ops.syncDir = func(f *os.File) error {
		data, err := os.ReadFile(filepath.Join(f.Name(), SnapshotName))
		if err != nil {
			return err
		}
		state, err := decodeSnapshot(data)
		if err != nil {
			return err
		}
		shouldBlock := state.Parallel == 3 || len(state.Jobs) == 1 && state.Jobs[0].State == Queued
		if shouldBlock && blocked.CompareAndSwap(false, true) {
			entered <- struct{}{}
			<-release
		}
		return f.Sync()
	}
	return entered, release
}

func waitCommit(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-testContext(t).Done():
		t.Fatal("mutation did not enter its durable commit")
	}
}
