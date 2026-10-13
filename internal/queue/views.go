package queue

import (
	"cmp"
	"slices"
)

type Status struct {
	State    string `json:"state"`
	Parallel int    `json:"parallel"`
	Active   int    `json:"active"`
}

type JobView struct {
	ID        string  `json:"id"`
	Order     int64   `json:"order"`
	State     State   `json:"state"`
	Output    string  `json:"output"`
	Published *bool   `json:"published"`
	Error     *string `json:"error"`
	Bytes     *int64  `json:"bytes"`
	Total     *int64  `json:"total"`
	Workers   *int    `json:"workers"`
}

type ListView struct {
	Daemon Status    `json:"daemon"`
	Jobs   []JobView `json:"jobs"`
}

func (c *Coordinator) status() Status {
	state := "running"
	if c.stopping {
		state = "stopping"
	}
	return Status{State: state, Parallel: c.state.Parallel, Active: len(c.active)}
}

func (c *Coordinator) view() ListView {
	view := ListView{Daemon: c.status(), Jobs: make([]JobView, 0, len(c.state.Jobs))}
	for _, j := range c.state.Jobs {
		view.Jobs = append(view.Jobs, c.jobView(j))
	}
	slices.SortFunc(view.Jobs, func(a, b JobView) int { return cmp.Compare(a.Order, b.Order) })
	return view
}

func (c *Coordinator) jobView(j Job) JobView {
	view := JobView{
		ID: j.ID, Order: j.Order, State: j.State, Output: j.Request.Output,
		Published: copyPtr(j.Published), Error: publicError(j),
	}
	s, observed := c.latest[j.ID]
	if a := c.active[j.ID]; a != nil {
		if progress := a.progress.Load(); progress != nil {
			s, observed = *progress, true
		}
	}
	if observed {
		if s.Bytes >= 0 {
			view.Bytes = &s.Bytes
		}
		if s.Total >= 0 {
			view.Total = &s.Total
		}
		if s.Workers >= 0 {
			view.Workers = &s.Workers
		}
	}
	return view
}
