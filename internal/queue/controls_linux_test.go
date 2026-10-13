package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"yoinkdl/internal/transfer"
)

func TestQueueLiveConcurrencyBoundsAndPersists(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	state := testSnapshot(t, dir, MaxParallel+2)
	must(t, s.Save(state))
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	attempts := []*fakeAttempt{f.next(t)}
	f.assertIdle(t)
	_, err = c.SetParallel(testContext(t), MaxParallel)
	must(t, err)
	for len(attempts) < MaxParallel {
		attempts = append(attempts, f.next(t))
	}
	view := listQueue(t, c)
	if view.Daemon.Active != MaxParallel || view.Jobs[MaxParallel].State != Queued {
		t.Fatal("parallel queue exceeded its admission limit")
	}
	f.assertIdle(t)
	status, err := c.SetParallel(testContext(t), 1)
	must(t, err)
	if status.Parallel != 1 || status.Active != MaxParallel {
		t.Fatal("live reduction hid active work")
	}
	for _, a := range attempts {
		if a.req.Connections != transfer.DefaultConnections {
			t.Fatal("queue concurrency changed per-transfer workers")
		}
		select {
		case <-a.ctx.Done():
			t.Fatal("live reduction canceled an active transfer")
		default:
		}
	}
	for i, a := range attempts[:MaxParallel-1] {
		a.exit <- outcome{result: transfer.Result{Published: true}}
		waitQueue(t, c, func(v ListView) bool { return v.Daemon.Active == MaxParallel-i-1 })
		f.assertIdle(t)
	}
	attempts[MaxParallel-1].exit <- outcome{result: transfer.Result{Published: true}}
	a := f.next(t)
	if a.req.Output != state.Jobs[MaxParallel].Request.Output {
		t.Fatal("remaining FIFO work was reordered after reduction")
	}
	if listQueue(t, c).Daemon.Active != 1 {
		t.Fatal("reduced concurrency did not govern new admissions")
	}
	f.assertIdle(t)
	_, err = c.Stop(testContext(t))
	must(t, err)
	a.exit <- outcome{err: context.Canceled}
	must(t, waitRun(t, done))
	must(t, s.Close())
	s = testStore(t, dir)
	restored, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	if restored.state.Parallel != 1 {
		t.Fatal("live setting did not survive restart")
	}
}

func TestQueueIdleControlsDeduplicationAndReservations(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	first := addJob(t, c, testJob(t, dir, 1))
	a := f.next(t)
	second := addJob(t, c, testJob(t, dir, 2))
	third := addJob(t, c, testJob(t, dir, 3))
	if duplicate := addJob(t, c, testJob(t, dir, 2)); duplicate.Order != second.Order {
		t.Fatal("duplicate add consumed another order")
	}
	changed, err := testJob(t, dir, 2).Request.Transfer()
	must(t, err)
	changed.URL += "&changed=queue-private"
	if _, err := c.Add(testContext(t), second.ID, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed request reused a retained ID")
	}
	if _, err := c.Forget(testContext(t), first.ID); !errors.Is(err, ErrBusy) {
		t.Fatal("active job was forgotten")
	}
	if j, err := c.Resume(testContext(t), first.ID); err != nil || j.Order != first.Order {
		t.Fatal("running resume was not idempotent")
	}
	paused, err := c.Pause(testContext(t), second.ID)
	must(t, err)
	if paused.State != Paused {
		t.Fatal("waiting job did not pause immediately")
	}
	for _, path := range []string{third.Output, third.Output + ".yoinkdl.part", third.Output + ".yoinkdl.json"} {
		writeFile(t, path, []byte("preserve"))
	}
	canceled, err := c.Cancel(testContext(t), third.ID)
	must(t, err)
	if canceled.State != Canceled || addJob(t, c, testJob(t, dir, 3)).State != Canceled {
		t.Fatal("terminal add deduplication lost the canceled record")
	}
	conflict, err := testJob(t, dir, 3).Request.Transfer()
	must(t, err)
	if _, err := c.Add(testContext(t), fmt.Sprintf("%032x", 4), conflict); !errors.Is(err, ErrConflict) {
		t.Fatal("terminal output reservation was bypassed")
	}
	for _, want := range []bool{true, false} {
		forgotten, err := c.Forget(testContext(t), third.ID)
		if err != nil || forgotten != want {
			t.Fatal("forget was not idempotent")
		}
	}
	for _, path := range []string{third.Output, third.Output + ".yoinkdl.part", third.Output + ".yoinkdl.json"} {
		if string(readFile(t, path)) != "preserve" {
			t.Fatal("cancel/forget removed transfer files")
		}
	}
	third = addJob(t, c, testJob(t, dir, 3))
	resumed, err := c.Resume(testContext(t), second.ID)
	must(t, err)
	if resumed.Order <= third.Order || resumed.State != Queued {
		t.Fatal("resume did not move to the FIFO tail")
	}
	if again, err := c.Resume(testContext(t), second.ID); err != nil || again.Order != resumed.Order {
		t.Fatal("queued resume consumed another order")
	}
	a.exit <- outcome{result: transfer.Result{Published: true}}
	b := f.next(t)
	if b.req.Output != third.Output || addJob(t, c, testJob(t, dir, 1)).State != Completed {
		t.Fatal("FIFO-tail resume or completed deduplication failed")
	}
	b.exit <- outcome{result: transfer.Result{Published: true}}
	b = f.next(t)
	if b.req.Output != second.Output {
		t.Fatal("resumed job was not admitted at the tail")
	}
	b.exit <- outcome{result: transfer.Result{Published: true}}
	waitQueue(t, c, func(v ListView) bool { return v.Daemon.Active == 0 })
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
}

func TestQueueControlsHoldSlotsAndPreservePublication(t *testing.T) {
	for _, op := range []string{"pause", "cancel"} {
		for _, result := range []struct {
			name      string
			published bool
			err       error
			state     State
		}{
			{"canceled", false, context.Canceled, Paused},
			{"completed", true, nil, Completed},
			{"published failure", true, errors.New("queue-private"), Failed},
			{"unpublished failure", false, errors.New("queue-private"), Failed},
		} {
			t.Run(op+"/"+result.name, func(t *testing.T) {
				dir := privateDir(t)
				s := testStore(t, dir)
				state := testSnapshot(t, dir, 2)
				must(t, s.Save(state))
				f := newFakeRunner(t)
				c, err := NewCoordinator(s, f.run, 0)
				must(t, err)
				done := runCoordinator(t, c, f)
				a := f.next(t)
				id := state.Jobs[0].ID
				control := c.Pause
				intent := Pausing
				if op == "cancel" {
					control, intent = c.Cancel, Canceling
				}
				view, err := control(testContext(t), id)
				must(t, err)
				waitCanceled(t, a.ctx)
				stored, err := s.Snapshot()
				must(t, err)
				if view.State != intent || stored.Jobs[0].State != intent || listQueue(t, c).Daemon.Active != 1 {
					t.Fatal("control canceled before intent or released its slot early")
				}
				if _, err := c.Forget(testContext(t), id); !errors.Is(err, ErrBusy) {
					t.Fatal("draining attempt was forgotten")
				}
				if _, err := c.Resume(testContext(t), id); !errors.Is(err, ErrBusy) {
					t.Fatal("draining attempt was resumed")
				}
				f.assertIdle(t)
				a.exit <- outcome{result: transfer.Result{Published: result.published, Bytes: 10}, err: result.err}
				b := f.next(t)
				want := result.state
				if op == "cancel" && want == Paused {
					want = Canceled
				}
				view = listQueue(t, c).Jobs[0]
				if view.State != want || view.Published == nil || *view.Published != result.published || view.Workers == nil || *view.Workers != 0 {
					t.Fatal("engine result was lost at the control boundary")
				}
				stored, err = s.Snapshot()
				must(t, err)
				if (stored.Jobs[0].Request.URL == nil) != (want == Completed || want == Canceled) {
					t.Fatal("terminal source retention was incorrect")
				}
				_, err = c.Stop(testContext(t))
				must(t, err)
				b.exit <- outcome{err: context.Canceled}
				must(t, waitRun(t, done))
			})
		}
	}
}

func TestQueueCancelOverridesPauseAndStoppingRefusesMutations(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	job := testJob(t, dir, 1)
	addJob(t, c, job)
	a := f.next(t)
	_, err = c.Pause(testContext(t), job.ID)
	must(t, err)
	for range 2 {
		view, err := c.Cancel(testContext(t), job.ID)
		if err != nil || view.State != Canceling {
			t.Fatal("cancel did not replace pause intent idempotently")
		}
	}
	_, err = c.Stop(testContext(t))
	must(t, err)
	for _, control := range []func(context.Context, string) (JobView, error){c.Pause, c.Resume, c.Cancel} {
		if _, err := control(testContext(t), job.ID); !errors.Is(err, ErrStopping) {
			t.Fatal("stopping queue accepted a job control")
		}
	}
	req, err := job.Request.Transfer()
	must(t, err)
	if _, err := c.Add(testContext(t), job.ID, req); !errors.Is(err, ErrStopping) {
		t.Fatal("stopping queue accepted add")
	}
	if _, err := c.Forget(testContext(t), job.ID); !errors.Is(err, ErrStopping) {
		t.Fatal("stopping queue accepted forget")
	}
	if _, err := c.SetParallel(testContext(t), 2); !errors.Is(err, ErrStopping) {
		t.Fatal("stopping queue accepted a setting")
	}
	if listQueue(t, c).Jobs[0].State != Canceling {
		t.Fatal("shutdown replaced existing cancel intent")
	}
	a.exit <- outcome{err: context.Canceled}
	must(t, waitRun(t, done))
	stored, err := s.Snapshot()
	must(t, err)
	if stored.Jobs[0].State != Canceled || stored.Jobs[0].Request.URL != nil {
		t.Fatal("cancel intent did not survive shutdown")
	}
}

func TestQueueControlValidationDoesNotMutate(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	before := readFile(t, filepath.Join(dir, SnapshotName))
	id := fmt.Sprintf("%032x", 1)
	for _, control := range []func(context.Context, string) (JobView, error){c.Pause, c.Resume, c.Cancel} {
		if _, err := control(testContext(t), "queue-private"); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid ID was accepted")
		}
		if _, err := control(testContext(t), id); !errors.Is(err, ErrNotFound) {
			t.Fatal("unknown ID was accepted")
		}
	}
	for _, parallel := range []int{-1, 0, MaxParallel + 1} {
		if _, err := c.SetParallel(testContext(t), parallel); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid concurrency setting was accepted")
		}
	}
	req, err := testJob(t, dir, 1).Request.Transfer()
	must(t, err)
	req.Overwrite = true
	if _, err := c.Add(testContext(t), id, req); !errors.Is(err, ErrInvalid) {
		t.Fatal("queued overwrite was accepted")
	}
	req.Overwrite = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Add(ctx, id, req); !errors.Is(err, context.Canceled) || errors.Is(err, ErrOutcomeUnknown) {
		t.Fatal("unsubmitted canceled request reported an uncertain mutation")
	}
	if string(readFile(t, filepath.Join(dir, SnapshotName))) != string(before) || len(listQueue(t, c).Jobs) != 0 {
		t.Fatal("invalid control mutated the queue")
	}
	if _, err := os.Stat(req.Output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid control touched output files")
	}
	f.assertIdle(t)
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
}

func addJob(t *testing.T, c *Coordinator, j Job) JobView {
	t.Helper()
	req, err := j.Request.Transfer()
	must(t, err)
	view, err := c.Add(testContext(t), j.ID, req)
	must(t, err)
	return view
}
