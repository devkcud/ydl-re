package queue

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func TestQueueCapacityAndOrderRefusal(t *testing.T) {
	for _, kind := range []string{"retained count", "order counter"} {
		t.Run(kind, func(t *testing.T) {
			dir := privateDir(t)
			s := testStore(t, dir)
			count := MaxJobs
			want := ErrCapacity
			if kind == "order counter" {
				count, want = 1, ErrOrder
			}
			state := testSnapshot(t, dir, count)
			for i := range state.Jobs {
				state.Jobs[i].State = Paused
			}
			if kind == "order counter" {
				state.NextOrder = math.MaxInt64
			}
			must(t, s.Save(state))
			f := newFakeRunner(t)
			c, err := NewCoordinator(s, f.run, 0)
			must(t, err)
			done := runCoordinator(t, c, f)
			job := testJob(t, dir, int64(count+1))
			req, err := job.Request.Transfer()
			must(t, err)
			if _, err := c.Add(testContext(t), job.ID, req); !errors.Is(err, want) {
				t.Fatalf("exhausted queue accepted add: %v", err)
			}
			if addJob(t, c, state.Jobs[0]).State != Paused {
				t.Fatal("capacity incorrectly blocked idempotent add")
			}
			if kind == "order counter" {
				if _, err := c.Resume(testContext(t), state.Jobs[0].ID); !errors.Is(err, ErrOrder) {
					t.Fatal("exhausted order counter allowed resume")
				}
				assertStore(t, s, state)
			} else {
				forgotten, err := c.Forget(testContext(t), state.Jobs[0].ID)
				if err != nil || !forgotten {
					t.Fatal("full queue could not free capacity")
				}
				addJob(t, c, job)
				a := f.next(t)
				_, err = c.Pause(testContext(t), job.ID)
				must(t, err)
				a.exit <- outcome{err: context.Canceled}
				waitQueue(t, c, func(v ListView) bool { return v.Daemon.Active == 0 })
			}
			f.assertIdle(t)
			_, err = c.Stop(testContext(t))
			must(t, err)
			must(t, waitRun(t, done))
		})
	}
}

func TestQueueEncodedCapacityIsRefusedBeforePersistence(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	source := "https://example.invalid/?"
	source += strings.Repeat("&", MaxURLSize-len(source))
	sample := testJob(t, dir, MaxJobs)
	sample.Request.URL = &source
	sample.InputHash, _ = sample.Request.hash()
	sample.State = Paused
	data, err := json.Marshal(sample)
	must(t, err)
	count := (MaxSnapshotSize - 128) / (len(data) + 1)
	state := testSnapshot(t, dir, count)
	for i := range state.Jobs {
		state.Jobs[i].State = Paused
		state.Jobs[i].Request.URL = &source
		state.Jobs[i].InputHash, err = state.Jobs[i].Request.hash()
		must(t, err)
	}
	must(t, s.Save(state))
	s.ops.write = func(*os.File, []byte) (int, error) {
		t.Fatal("oversized candidate reached disk IO")
		return 0, nil
	}
	f := newFakeRunner(t)
	c, err := NewCoordinator(s, f.run, 0)
	must(t, err)
	done := runCoordinator(t, c, f)
	job := testJob(t, dir, int64(count+1))
	req, err := job.Request.Transfer()
	must(t, err)
	req.URL = source
	if _, err := c.Add(testContext(t), job.ID, req); !errors.Is(err, ErrCapacity) {
		t.Fatalf("encoded capacity was not refused: %v", err)
	}
	view := listQueue(t, c)
	if view.Daemon.State != "running" || len(view.Jobs) != count {
		t.Fatal("input capacity refusal stopped or mutated the queue")
	}
	assertStore(t, s, state)
	f.assertIdle(t)
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
}
