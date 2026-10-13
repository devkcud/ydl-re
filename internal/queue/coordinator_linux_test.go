package queue

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"yoinkdl/internal/transfer"
)

func TestCoordinatorRunsFIFOAndWaitsForResults(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	state := testSnapshot(t, dir, 3)
	slices.Reverse(state.Jobs)
	must(t, s.Save(state))
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)

	for order := 1; order <= 3; order++ {
		a := f.next(t)
		want := testJob(t, dir, int64(order))
		if a.req.Output != want.Request.Output || a.req.Overwrite {
			t.Fatal("FIFO or no-overwrite policy was lost")
		}
		stored, err := s.Snapshot()
		must(t, err)
		for _, j := range stored.Jobs {
			if j.ID == want.ID && (j.State != Running || j.Published != nil) {
				t.Fatal("engine started before running intent was committed")
			}
		}
		view := listQueue(t, c)
		if view.Daemon.Active != 1 || view.Daemon.Parallel != 1 || view.Jobs[order-1].State != Running {
			t.Fatal("sequential queue admitted excess work or returned unsorted jobs")
		}
		f.assertIdle(t)
		a.exit <- outcome{result: transfer.Result{Published: true, Bytes: 12}}
	}
	waitQueue(t, c, func(v ListView) bool { return v.Daemon.Active == 0 })
	if err := c.Run(context.Background()); !errors.Is(err, ErrStarted) {
		t.Fatal("coordinator ran twice")
	}
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
	if _, err := c.List(testContext(t)); !errors.Is(err, ErrStopped) {
		t.Fatal("stopped coordinator accepted a read")
	}
	other, err := Open(dir)
	if other != nil || !errors.Is(err, ErrLocked) {
		t.Fatal("coordinator released caller-owned storage")
	}
	stored, err := s.Snapshot()
	must(t, err)
	for _, j := range stored.Jobs {
		if j.State != Completed || j.Request.URL != nil || j.Published == nil || !*j.Published {
			t.Fatal("finished transfer was not recorded durably")
		}
	}
}

func TestCoordinatorPreparesRecoveryBeforeDispatch(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	state := testSnapshot(t, dir, 5)
	for i, kind := range []State{Running, Pausing, Canceling, Queued, Failed} {
		state.Jobs[i].State = kind
		if i < 3 {
			state.Jobs[i].Published = nil
		}
		if kind == Failed {
			state.Jobs[i].Error = ptr("https://example.invalid/?token=queue-private")
		}
	}
	must(t, s.Save(state))
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 3)
	must(t, err)
	f.assertIdle(t)
	state.Parallel = 3
	recoverJobs(&state)
	assertStore(t, s, state)
	done := runCoordinator(t, c, f)
	a := f.next(t)
	if a.req.Output != state.Jobs[3].Request.Output {
		t.Fatal("recovered interrupted attempt was replayed")
	}
	view := listQueue(t, c)
	if view.Daemon.Active != 1 || view.Daemon.Parallel != 3 || view.Jobs[0].Bytes != nil || view.Jobs[0].Published != nil {
		t.Fatal("recovery invented progress or ignored the saved setting")
	}
	f.assertIdle(t)
	data, err := json.Marshal(view)
	must(t, err)
	for _, text := range []string{"queue-private", "https://", "input_hash", "\"request\"", "\"url\""} {
		if strings.Contains(string(data), text) {
			t.Fatal("private request data reached a queue view")
		}
	}
	*view.Jobs[4].Published = true
	*view.Jobs[4].Error = "changed"
	if got := listQueue(t, c).Jobs[4]; *got.Published || *got.Error == "changed" {
		t.Fatal("public view aliases internal state")
	}
	a.exit <- outcome{err: errors.New("queue-private")}
	waitQueue(t, c, func(v ListView) bool { return v.Daemon.Active == 0 })
	f.assertIdle(t)
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
}

func TestCoordinatorContextShutdownPersistsBeforeCancellation(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	state := testSnapshot(t, dir, 3)
	state.Parallel = 2
	must(t, s.Save(state))
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := runCoordinatorContext(t, c, ctx, cancel, f)
	a, b := f.next(t), f.next(t)
	cancel()
	waitCanceled(t, a.ctx)
	waitCanceled(t, b.ctx)
	stored, err := s.Snapshot()
	must(t, err)
	if stored.Jobs[0].State != Pausing || stored.Jobs[1].State != Pausing || stored.Jobs[2].State != Queued {
		t.Fatal("contexts were canceled before durable pause intent")
	}
	select {
	case <-c.Done():
		t.Fatal("shutdown did not wait for engine cleanup")
	default:
	}
	view := listQueue(t, c)
	if view.Daemon.State != "stopping" || view.Daemon.Active != 2 {
		t.Fatal("draining transfers lost their slots")
	}
	a.exit <- outcome{result: transfer.Result{Published: true}}
	waitQueue(t, c, func(v ListView) bool { return v.Daemon.Active == 1 })
	f.assertIdle(t)
	b.exit <- outcome{err: context.Canceled}
	if err := waitRun(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("signal outcome was lost: %v", err)
	}
	stored, err = s.Snapshot()
	must(t, err)
	states := []State{stored.Jobs[0].State, stored.Jobs[1].State}
	slices.Sort(states)
	if !reflect.DeepEqual(states, []State{Completed, Paused}) || stored.Jobs[2].State != Queued {
		t.Fatal("shutdown lost actual engine outcomes or consumed waiting work")
	}
}

func TestCoordinatorCanceledBeforeRunDoesNotAdmit(t *testing.T) {
	s := testStore(t, privateDir(t))
	state := testSnapshot(t, t.TempDir(), 1)
	must(t, s.Save(state))
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-canceled coordinator did not stop")
	}
	f.assertIdle(t)
	assertStore(t, s, state)
}

func TestCoordinatorRejectsInvalidStartupWithoutMutation(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	before := readFile(t, filepath.Join(dir, SnapshotName))
	for _, parallel := range []int{-1, MaxParallel + 1} {
		c, err := NewCoordinator(s, nil, parallel)
		if c != nil || !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid startup setting was accepted")
		}
	}
	if c, err := NewCoordinator(nil, nil, 0); c != nil || !errors.Is(err, ErrInvalid) {
		t.Fatal("nil store was accepted")
	}
	if !slices.Equal(before, readFile(t, filepath.Join(dir, SnapshotName))) {
		t.Fatal("invalid startup modified the queue")
	}
	must(t, s.Close())
	if c, err := NewCoordinator(s, nil, 0); c != nil || !errors.Is(err, os.ErrClosed) || !errors.Is(err, ErrPersistence) {
		t.Fatal("closed store was accepted")
	}
}

type fakeAttempt struct {
	ctx      context.Context
	req      transfer.Request
	progress func(transfer.Snapshot)
	exit     chan outcome
}

type fakeRunner struct {
	calls   chan *fakeAttempt
	release chan struct{}
	once    sync.Once
}

func newFakeRunner(t *testing.T) *fakeRunner {
	t.Helper()
	f := &fakeRunner{calls: make(chan *fakeAttempt, MaxParallel), release: make(chan struct{})}
	t.Cleanup(f.stop)
	return f
}

func (f *fakeRunner) run(ctx context.Context, req transfer.Request, progress func(transfer.Snapshot)) (transfer.Result, error) {
	a := &fakeAttempt{ctx: ctx, req: req, progress: progress, exit: make(chan outcome, 1)}
	f.calls <- a
	select {
	case r := <-a.exit:
		return r.result, r.err
	case <-f.release:
		return transfer.Result{}, context.Canceled
	}
}

func (f *fakeRunner) stop() {
	f.once.Do(func() { close(f.release) })
}

func (f *fakeRunner) next(t *testing.T) *fakeAttempt {
	t.Helper()
	select {
	case a := <-f.calls:
		return a
	case <-testContext(t).Done():
		t.Fatal("expected transfer did not start")
		return nil
	}
}

func (f *fakeRunner) assertIdle(t *testing.T) {
	t.Helper()
	select {
	case <-f.calls:
		t.Fatal("unexpected transfer started")
	default:
	}
}

func runCoordinator(t *testing.T, c *Coordinator, runners ...*fakeRunner) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	return runCoordinatorContext(t, c, ctx, cancel, runners...)
}

func runCoordinatorContext(t *testing.T, c *Coordinator, ctx context.Context, cancel context.CancelFunc, runners ...*fakeRunner) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		for _, f := range runners {
			f.stop()
		}
		select {
		case <-c.Done():
		case <-time.After(5 * time.Second):
			t.Error("coordinator leaked after test cleanup")
		}
	})
	return done
}

func waitRun(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-testContext(t).Done():
		t.Fatal("coordinator did not join its transfers")
		return nil
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func waitCanceled(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-testContext(t).Done():
		t.Fatal("transfer context was not canceled")
	}
}

func listQueue(t *testing.T, c *Coordinator) ListView {
	t.Helper()
	view, err := c.List(testContext(t))
	must(t, err)
	return view
}

func waitQueue(t *testing.T, c *Coordinator, ready func(ListView) bool) ListView {
	t.Helper()
	ctx := testContext(t)
	for {
		view, err := c.List(ctx)
		must(t, err)
		if ready(view) {
			return view
		}
	}
}
