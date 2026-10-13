package queue

import (
	"context"
	"errors"
	"sync/atomic"

	"yoinkdl/internal/transfer"
)

const MaxControls = 16

var ErrInvalid = errors.New("invalid queue request or setting")
var ErrStopped = errors.New("queue coordinator has stopped")
var ErrStarted = errors.New("queue coordinator can run only once")
var ErrMailbox = errors.New("queue control mailbox is full")
var ErrOutcomeUnknown = errors.New("queue mutation outcome is unknown; inspect queue state")
var ErrPersistence = errors.New("queue persistence failed; stop and reopen the queue")

type Runner func(context.Context, transfer.Request, func(transfer.Snapshot)) (transfer.Result, error)

type Coordinator struct {
	store    *Store
	run      Runner
	state    Snapshot
	active   map[string]*attempt
	latest   map[string]transfer.Snapshot
	calls    chan command
	results  chan outcome
	done     chan struct{}
	started  atomic.Bool
	stopping bool
	fault    error
}

type attempt struct {
	cancel   context.CancelFunc
	progress atomic.Pointer[transfer.Snapshot]
}

type outcome struct {
	id     string
	result transfer.Result
	err    error
}

type operation uint8

const (
	listOp operation = iota
	stopOp
)

type command struct {
	op    operation
	reply chan response
}

type response struct {
	list   ListView
	status Status
	err    error
}

type persistenceError struct {
	cause error
}

func (e *persistenceError) Error() string {
	return ErrPersistence.Error()
}

func (e *persistenceError) Unwrap() error {
	return e.cause
}

func (e *persistenceError) Is(target error) bool {
	return target == ErrPersistence
}

func NewCoordinator(store *Store, run Runner, parallel int) (*Coordinator, error) {
	if store == nil || parallel < 0 || parallel > MaxParallel {
		return nil, ErrInvalid
	}

	s, err := store.Snapshot()
	if err != nil {
		return nil, &persistenceError{cause: err}
	}
	changed := recoverJobs(&s)
	if parallel != 0 && parallel != s.Parallel {
		s.Parallel = parallel
		changed = true
	}
	if changed {
		if err := store.Save(s); err != nil {
			return nil, &persistenceError{cause: err}
		}
	}
	if run == nil {
		run = transfer.Download
	}

	return &Coordinator{
		store: store, run: run, state: s,
		active: make(map[string]*attempt), latest: make(map[string]transfer.Snapshot),
		calls: make(chan command, MaxControls), results: make(chan outcome, MaxParallel), done: make(chan struct{}),
	}, nil
}

func (c *Coordinator) Run(ctx context.Context) error {
	if !c.started.CompareAndSwap(false, true) {
		return ErrStarted
	}
	defer close(c.done)

	stop := ctx.Done()
	var reason error
	for {
		if stop != nil && ctx.Err() != nil {
			reason = ctx.Err()
			stop = nil
			c.stop()
		}
		c.dispatch(ctx)
		if c.stopping && len(c.active) == 0 {
			if c.fault != nil {
				return c.fault
			}
			return reason
		}

		select {
		case cmd := <-c.calls:
			cmd.reply <- c.handle(cmd)
		case result := <-c.results:
			c.finish(result)
		case <-stop:
			reason = ctx.Err()
			stop = nil
			c.stop()
		}
	}
}

func (c *Coordinator) Done() <-chan struct{} {
	return c.done
}

func (c *Coordinator) List(ctx context.Context) (ListView, error) {
	r := c.call(ctx, command{op: listOp})
	return r.list, r.err
}

func (c *Coordinator) Stop(ctx context.Context) (Status, error) {
	r := c.call(ctx, command{op: stopOp})
	return r.status, r.err
}

func (c *Coordinator) call(ctx context.Context, cmd command) response {
	if err := ctx.Err(); err != nil {
		return response{err: err}
	}
	select {
	case <-c.done:
		return response{err: c.closedError()}
	default:
	}

	cmd.reply = make(chan response, 1)
	select {
	case c.calls <- cmd:
	case <-ctx.Done():
		return response{err: ctx.Err()}
	case <-c.done:
		return response{err: c.closedError()}
	default:
		return response{err: ErrMailbox}
	}

	select {
	case r := <-cmd.reply:
		return r
	case <-ctx.Done():
		select {
		case r := <-cmd.reply:
			return r
		default:
		}
		if cmd.op != listOp {
			return response{err: errors.Join(ErrOutcomeUnknown, ctx.Err())}
		}
		return response{err: ctx.Err()}
	case <-c.done:
		select {
		case r := <-cmd.reply:
			return r
		default:
			return response{err: c.closedError()}
		}
	}
}

func (c *Coordinator) closedError() error {
	if c.fault != nil {
		return c.fault
	}
	return ErrStopped
}

func (c *Coordinator) handle(cmd command) response {
	if c.fault != nil {
		return response{err: c.fault}
	}
	switch cmd.op {
	case listOp:
		return response{list: c.view()}
	case stopOp:
		if err := c.stop(); err != nil {
			return response{err: err}
		}
		return response{status: c.status()}
	default:
		return response{err: ErrInvalid}
	}
}

func (c *Coordinator) dispatch(ctx context.Context) {
	for !c.stopping && len(c.active) < c.state.Parallel && ctx.Err() == nil {
		i := -1
		for n, j := range c.state.Jobs {
			if j.State == Queued && (i == -1 || j.Order < c.state.Jobs[i].Order) {
				i = n
			}
		}
		if i == -1 {
			return
		}

		next := c.state.clone()
		j := &next.Jobs[i]
		j.State = Running
		j.Published = nil
		j.Error = nil
		req, err := j.Request.Transfer()
		if err != nil {
			c.fail(err)
			return
		}
		if err := c.persist(next); err != nil {
			return
		}

		ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		a := &attempt{cancel: cancel}
		id := j.ID
		c.active[id] = a
		delete(c.latest, id)
		go func() {
			result, err := c.run(ctx, req, func(s transfer.Snapshot) { a.progress.Store(&s) })
			cancel()
			c.results <- outcome{id: id, result: result, err: err}
		}()
	}
}

func (c *Coordinator) finish(done outcome) {
	a := c.active[done.id]
	if c.fault == nil {
		next := c.state.clone()
		for i := range next.Jobs {
			if next.Jobs[i].ID == done.id {
				next.Jobs[i] = finishJob(next.Jobs[i], done.result, done.err)
				break
			}
		}
		c.persist(next)
	}

	s := transfer.Snapshot{Total: -1}
	if progress := a.progress.Load(); progress != nil {
		s = *progress
	}
	s.Bytes = done.result.Bytes
	s.Workers = 0
	c.latest[done.id] = s
	delete(c.active, done.id)
}

func (c *Coordinator) stop() error {
	if c.fault != nil {
		return c.fault
	}
	if c.stopping {
		return nil
	}

	next := c.state.clone()
	changed := false
	for i := range next.Jobs {
		if next.Jobs[i].State == Running {
			next.Jobs[i].State = Pausing
			changed = true
		}
	}
	if changed {
		if err := c.persist(next); err != nil {
			return err
		}
	}
	c.stopping = true
	for _, a := range c.active {
		a.cancel()
	}
	return nil
}

func (c *Coordinator) persist(next Snapshot) error {
	if err := c.store.Save(next); err != nil {
		c.fail(err)
		return c.fault
	}
	c.state = next
	return nil
}

func (c *Coordinator) fail(err error) {
	if c.fault == nil {
		c.fault = &persistenceError{cause: err}
	}
	c.stopping = true
	for _, a := range c.active {
		a.cancel()
	}
}
