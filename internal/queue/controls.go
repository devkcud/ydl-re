package queue

import (
	"context"
	"errors"
	"math"
	"slices"

	"yoinkdl/internal/transfer"
)

var ErrStopping = errors.New("queue is stopping; mutations are disabled")
var ErrNotFound = errors.New("queue job was not found")
var ErrCapacity = errors.New("queue retention or snapshot capacity is exhausted")

func (c *Coordinator) Add(ctx context.Context, id string, req transfer.Request) (JobView, error) {
	j, err := NewJob(id, 1, req)
	if err != nil {
		return JobView{}, ErrInvalid
	}
	r := c.call(ctx, command{op: addOp, job: j})
	return r.job, r.err
}

func (c *Coordinator) Pause(ctx context.Context, id string) (JobView, error) {
	r := c.control(ctx, pauseOp, id)
	return r.job, r.err
}

func (c *Coordinator) Resume(ctx context.Context, id string) (JobView, error) {
	r := c.control(ctx, resumeOp, id)
	return r.job, r.err
}

func (c *Coordinator) Cancel(ctx context.Context, id string) (JobView, error) {
	r := c.control(ctx, cancelOp, id)
	return r.job, r.err
}

func (c *Coordinator) Forget(ctx context.Context, id string) (bool, error) {
	r := c.control(ctx, forgetOp, id)
	return r.forgotten, r.err
}

func (c *Coordinator) SetParallel(ctx context.Context, parallel int) (Status, error) {
	if parallel < 1 || parallel > MaxParallel {
		return Status{}, ErrInvalid
	}
	r := c.call(ctx, command{op: parallelOp, parallel: parallel})
	return r.status, r.err
}

func (c *Coordinator) control(ctx context.Context, op operation, id string) response {
	if !validHex(id, 16) {
		return response{err: ErrInvalid}
	}
	return c.call(ctx, command{op: op, id: id})
}

func (c *Coordinator) mutate(cmd command) response {
	switch cmd.op {
	case addOp:
		return c.add(cmd.job)
	case parallelOp:
		if cmd.parallel != c.state.Parallel {
			next := c.state.clone()
			next.Parallel = cmd.parallel
			if err := c.commit(next); err != nil {
				return response{err: err}
			}
		}
		return response{status: c.status()}
	case pauseOp, resumeOp, cancelOp, forgetOp:
	default:
		return response{err: ErrInvalid}
	}

	i := slices.IndexFunc(c.state.Jobs, func(j Job) bool { return j.ID == cmd.id })
	if i == -1 {
		if cmd.op == forgetOp {
			return response{}
		}
		return response{err: ErrNotFound}
	}
	if cmd.op == forgetOp && c.active[cmd.id] != nil {
		return response{err: ErrBusy}
	}

	next := c.state.clone()
	j := &next.Jobs[i]
	changed := true
	var err error
	switch cmd.op {
	case pauseOp:
		changed, err = pauseJob(j)
	case resumeOp:
		changed, err = resumeJob(j, &next.NextOrder)
	case cancelOp:
		changed, err = cancelJob(j)
	case forgetOp:
		next.Jobs = slices.Delete(next.Jobs, i, i+1)
	}
	if err != nil {
		return response{err: err}
	}
	if changed {
		if err := c.commit(next); err != nil {
			return response{err: err}
		}
	}
	if cmd.op == forgetOp {
		delete(c.latest, cmd.id)
		return response{forgotten: true}
	}
	if changed && (cmd.op == pauseOp || cmd.op == cancelOp) {
		if a := c.active[cmd.id]; a != nil {
			a.cancel()
		}
	}
	return response{job: c.jobView(c.state.Jobs[i])}
}

func (c *Coordinator) add(j Job) response {
	for _, old := range c.state.Jobs {
		if old.ID == j.ID {
			if old.InputHash != j.InputHash {
				return response{err: ErrConflict}
			}
			return response{job: c.jobView(old)}
		}
	}
	for _, old := range c.state.Jobs {
		if old.Request.Output == j.Request.Output {
			return response{err: ErrConflict}
		}
	}
	if len(c.state.Jobs) == MaxJobs {
		return response{err: ErrCapacity}
	}
	if c.state.NextOrder == math.MaxInt64 {
		return response{err: ErrOrder}
	}

	next := c.state.clone()
	j.Order = next.NextOrder
	next.NextOrder++
	next.Jobs = append(next.Jobs, j)
	if err := c.commit(next); err != nil {
		return response{err: err}
	}
	return response{job: c.jobView(j)}
}

func (c *Coordinator) commit(next Snapshot) error {
	if err := next.Validate(); err != nil {
		return ErrCapacity
	}
	return c.persist(next)
}
