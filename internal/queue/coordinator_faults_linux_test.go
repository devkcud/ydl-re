package queue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"yoinkdl/internal/transfer"
)

func TestCoordinatorFailedRecoveryNeverDispatches(t *testing.T) {
	s := testStore(t, privateDir(t))
	state := testSnapshot(t, t.TempDir(), 1)
	state.Jobs[0].State = Running
	state.Jobs[0].Published = nil
	must(t, s.Save(state))
	s.ops.write = func(*os.File, []byte) (int, error) { return 0, errors.New("queue-private") }
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 2)
	assertPrivateError(t, err)
	if c != nil || !errors.Is(err, ErrPersistence) {
		t.Fatal("failed reconciliation produced a runnable coordinator")
	}
	f.assertIdle(t)
	assertStore(t, s, state)
}

func TestCoordinatorFailedStartIntentNeverCallsEngine(t *testing.T) {
	s := testStore(t, privateDir(t))
	state := testSnapshot(t, t.TempDir(), 1)
	must(t, s.Save(state))
	s.ops.write = func(*os.File, []byte) (int, error) { return 0, errors.New("queue-private") }
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	err = c.Run(testContext(t))
	assertPrivateError(t, err)
	if !errors.Is(err, ErrPersistence) {
		t.Fatal("failed start intent did not stop the coordinator")
	}
	f.assertIdle(t)
	assertStore(t, s, state)
}

func TestCoordinatorCommitFailureCancelsJoinsAndNeverRollsBack(t *testing.T) {
	for _, phase := range []string{"result write", "result sync", "stop write"} {
		t.Run(phase, func(t *testing.T) {
			dir := privateDir(t)
			s := testStore(t, dir)
			state := testSnapshot(t, dir, 3)
			state.Parallel = 2
			must(t, s.Save(state))
			var fail atomic.Bool
			var writes atomic.Int32
			s.ops.write = func(f *os.File, data []byte) (int, error) {
				writes.Add(1)
				if fail.Load() && phase != "result sync" {
					return 0, errors.New("queue-private")
				}
				return f.Write(data)
			}
			s.ops.syncDir = func(f *os.File) error {
				if fail.Load() && phase == "result sync" {
					return errors.New("queue-private")
				}
				return f.Sync()
			}
			f := newFakeRunner(t)
			c, err := NewCoordinator(s, f.run, 0)
			must(t, err)
			done := runCoordinator(t, c, f)
			a, b := f.next(t), f.next(t)
			listQueue(t, c)
			fail.Store(true)
			if phase == "stop write" {
				_, err = c.Stop(testContext(t))
				if !errors.Is(err, ErrPersistence) {
					t.Fatal("failed pause intent was acknowledged")
				}
			} else {
				a.exit <- outcome{result: transfer.Result{Published: true}}
			}
			waitCanceled(t, b.ctx)
			select {
			case <-c.Done():
				t.Fatal("persistence failure skipped engine cleanup")
			default:
			}
			_, err = c.List(testContext(t))
			assertPrivateError(t, err)
			if !errors.Is(err, ErrPersistence) {
				t.Fatal("faulted coordinator returned an authoritative view")
			}
			if phase == "result sync" && !errors.Is(err, ErrUncertain) {
				t.Fatal("uncertain commit lost its error identity")
			}
			before := writes.Load()
			b.exit <- outcome{err: context.Canceled}
			if phase == "stop write" {
				a.exit <- outcome{err: context.Canceled}
			}
			if err := waitRun(t, done); !errors.Is(err, ErrPersistence) {
				t.Fatal("persistence failure was not returned after joining")
			}
			if writes.Load() != before {
				t.Fatal("faulted coordinator wrote a stale snapshot")
			}
			f.assertIdle(t)
			stored, err := decodeSnapshot(readFile(t, filepath.Join(dir, SnapshotName)))
			must(t, err)
			completed := 0
			for _, j := range stored.Jobs {
				if j.State == Completed {
					completed++
				}
			}
			want := 0
			if phase == "result sync" {
				want = 1
			}
			if completed != want || stored.Jobs[2].State != Queued {
				t.Fatal("failure lost canonical state or admitted more work")
			}
		})
	}
}

func TestCoordinatorDisconnectedStopAndProgressDuringCommit(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	state := testSnapshot(t, dir, 1)
	must(t, s.Save(state))
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)
	var block atomic.Bool
	s.ops.syncFile = func(f *os.File) error {
		if block.CompareAndSwap(true, false) {
			entered <- struct{}{}
			<-release
		}
		return f.Sync()
	}
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	a := f.next(t)
	listQueue(t, c)
	block.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reply := make(chan error, 1)
	go func() {
		_, err := c.Stop(ctx)
		reply <- err
	}()
	select {
	case <-entered:
	case <-testContext(t).Done():
		t.Fatal("stop did not begin its commit")
	}
	select {
	case <-a.ctx.Done():
		t.Fatal("transfer was canceled before stop intent became durable")
	default:
	}
	cancel()
	select {
	case err := <-reply:
		if !errors.Is(err, ErrOutcomeUnknown) || !errors.Is(err, context.Canceled) {
			t.Fatal("disconnected mutation incorrectly claimed rollback")
		}
	case <-testContext(t).Done():
		t.Fatal("canceled caller waited on filesystem IO")
	}

	pending := make([]chan response, MaxControls)
	for i := range pending {
		pending[i] = make(chan response, 1)
		c.calls <- command{op: listOp, reply: pending[i]}
	}
	if _, err := c.List(testContext(t)); !errors.Is(err, ErrMailbox) {
		t.Fatal("coordinator control backlog was not bounded")
	}
	progress := make(chan struct{})
	go func() {
		for i := range 10000 {
			a.progress(transfer.Snapshot{Bytes: int64(i), Total: -1, Workers: 4})
		}
		close(progress)
	}()
	select {
	case <-progress:
	case <-testContext(t).Done():
		t.Fatal("progress was blocked by persistence or control clients")
	}
	release <- struct{}{}
	waitCanceled(t, a.ctx)
	for _, reply := range pending {
		select {
		case <-reply:
		case <-testContext(t).Done():
			t.Fatal("abandoned replies stalled the coordinator")
		}
	}
	view := listQueue(t, c)
	if view.Daemon.State != "stopping" || view.Daemon.Active != 1 || view.Jobs[0].State != Pausing ||
		view.Jobs[0].Bytes == nil || *view.Jobs[0].Bytes != 9999 || view.Jobs[0].Total != nil {
		t.Fatal("committed stop or coalesced progress was lost")
	}
	_, err = c.Stop(testContext(t))
	must(t, err)
	a.exit <- outcome{result: transfer.Result{Bytes: 8000}, err: context.Canceled}
	must(t, waitRun(t, done))
	stored, err := s.Snapshot()
	must(t, err)
	if stored.Jobs[0].State != Paused {
		t.Fatal("disconnected stop did not finish its accepted operation")
	}
}
