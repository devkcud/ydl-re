package queue

import (
	"context"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"yoinkdl/internal/transfer"
)

func TestRecoveryPreservesUnknownOutcomes(t *testing.T) {
	s := NewSnapshot()
	states := []State{Queued, Running, Pausing, Paused, Canceling, Canceled, Interrupted, Failed, Completed}
	want := []State{Queued, Interrupted, Paused, Paused, Canceled, Canceled, Interrupted, Failed, Completed}
	for _, state := range states {
		j := stateJob(t, state)
		j.Order = int64(len(s.Jobs) + 1)
		s.Jobs = append(s.Jobs, j)
	}
	s.NextOrder = 10
	before := s.clone()
	if !recoverJobs(&s) {
		t.Fatal("active attempts were not recovered")
	}
	for i, j := range s.Jobs {
		if j.State != want[i] || j.InputHash != before.Jobs[i].InputHash || j.Order != before.Jobs[i].Order {
			t.Fatal("recovery changed identity, order, or the expected state")
		}
		if states[i] == Running || states[i] == Pausing || states[i] == Canceling {
			if j.Published != nil {
				t.Fatal("recovery invented a publication result")
			}
		} else if !reflect.DeepEqual(j, before.Jobs[i]) {
			t.Fatal("recovery altered an idle record")
		}
		if err := j.validate(); err != nil {
			t.Fatal(err)
		}
	}
	if recoverJobs(&s) || s.NextOrder != before.NextOrder || s.Parallel != before.Parallel {
		t.Fatal("recovery was not idempotent or changed settings")
	}
}

func TestQueueControlTransitions(t *testing.T) {
	for _, test := range []struct {
		state  State
		pause  State
		resume State
		cancel State
	}{
		{Queued, Paused, Queued, Canceled},
		{Running, Pausing, Running, Canceling},
		{Pausing, Pausing, "", Canceling},
		{Paused, Paused, Queued, Canceled},
		{Canceling, "", "", Canceling},
		{Canceled, "", "", Canceled},
		{Interrupted, "", Queued, Canceled},
		{Failed, "", Queued, Canceled},
		{Completed, "", "", ""},
	} {
		for _, op := range []string{"pause", "resume", "cancel"} {
			t.Run(string(test.state)+"/"+op, func(t *testing.T) {
				j := stateJob(t, test.state)
				before := j
				next := int64(9)
				var changed bool
				var err error
				var want State
				switch op {
				case "pause":
					changed, err = pauseJob(&j)
					want = test.pause
				case "resume":
					changed, err = resumeJob(&j, &next)
					want = test.resume
				case "cancel":
					changed, err = cancelJob(&j)
					want = test.cancel
				}

				if want == "" {
					if err == nil || changed || !reflect.DeepEqual(j, before) || next != 9 {
						t.Fatal("invalid transition mutated the record")
					}
					return
				}
				if err != nil || j.State != want || changed != (test.state != want) {
					t.Fatalf("unexpected transition: %s, %t, %v", j.State, changed, err)
				}
				if op == "resume" && changed {
					if next != 10 || j.Order != 9 {
						t.Fatal("resume did not move to the FIFO tail")
					}
				} else if next != 9 || j.Order != before.Order {
					t.Fatal("control unexpectedly consumed an order")
				}
				if !reflect.DeepEqual(j.Published, before.Published) || j.InputHash != before.InputHash {
					t.Fatal("control changed the last outcome or request identity")
				}
				if err := j.validate(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestResumeRefusesOrderOverflow(t *testing.T) {
	j := stateJob(t, Paused)
	before := j
	next := int64(math.MaxInt64)
	changed, err := resumeJob(&j, &next)
	if changed || !errors.Is(err, ErrOrder) || !reflect.DeepEqual(j, before) || next != math.MaxInt64 {
		t.Fatal("overflowing resume changed queue state")
	}
}

func TestFinalResultOverridesLateControls(t *testing.T) {
	for _, state := range []State{Running, Pausing, Canceling} {
		for _, test := range []struct {
			name      string
			published bool
			err       error
			message   string
		}{
			{"success", true, nil, ""},
			{"unpublished success", false, nil, unpublishedMessage},
			{"failure", false, errors.New("https://example.invalid/?token=queue-private"), failedMessage},
			{"published error", true, errors.New("queue-private"), publishedMessage},
			{"late cancel", true, context.Canceled, publishedMessage},
			{"cancellation", false, context.Canceled, canceledMessage},
			{"timeout", false, context.DeadlineExceeded, timeoutMessage},
			{"permissions", false, os.ErrPermission, accessMessage},
		} {
			t.Run(string(state)+"/"+test.name, func(t *testing.T) {
				j := stateJob(t, state)
				got := finishJob(j, transfer.Result{Published: test.published}, test.err)
				want := Failed
				message := test.message
				if test.err == nil && test.published {
					want = Completed
				} else if test.err == context.Canceled && !test.published && state != Running {
					want, message = Paused, ""
					if state == Canceling {
						want = Canceled
					}
				}
				if got.State != want || got.Published == nil || *got.Published != test.published || got.InputHash != j.InputHash {
					t.Fatal("final result lost the actual publication outcome")
				}
				if (got.Error == nil) != (message == "") || got.Error != nil && *got.Error != message {
					t.Fatal("final result returned an unsafe or incorrect error")
				}
				if err := got.validate(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPublicErrorNeverReflectsStoredPayloads(t *testing.T) {
	for _, state := range []State{Failed, Interrupted} {
		j := stateJob(t, state)
		j.Error = ptr("https://example.invalid/?token=queue-private")
		message := publicError(j)
		if message == nil || strings.Contains(*message, "queue-private") || strings.Contains(*message, "https://") {
			t.Fatal("stored diagnostic reached a public view")
		}
		*message = "changed"
		if *j.Error == "changed" {
			t.Fatal("public error aliases private storage")
		}
	}
}

func stateJob(t *testing.T, state State) Job {
	t.Helper()
	j := testJob(t, t.TempDir(), 1)
	j.State = state
	switch state {
	case Running, Pausing, Canceling, Interrupted:
		j.Published = nil
	case Completed:
		j.Published = ptr(true)
	}
	if state == Failed || state == Interrupted {
		j.Error = ptr("previous failure")
	}
	if state == Completed || state == Canceled {
		j.Request.URL = nil
	}
	return j
}
