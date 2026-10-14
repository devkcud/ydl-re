package daemon

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

type Server struct {
	owner *owner
	queue *queue.Coordinator
	limit int64
	done  chan struct{}
	stop  chan struct{}
	fault chan error
	once  sync.Once
	state atomic.Uint32
	err   error
}

func New(opts Options) (*Server, error) {
	return newServer(opts, nil)
}

func newServer(opts Options, run queue.Runner) (_ *Server, err error) {
	if opts.Parallel < 0 || opts.Parallel > queue.MaxParallel || opts.Limit < 0 {
		return nil, queue.ErrInvalid
	}
	budget, err := transfer.NewBudget(opts.Limit)
	if err != nil {
		return nil, err
	}
	if run == nil {
		run = budget.Download
	}
	o, err := acquire(opts.Paths)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, o.close())
		}
	}()
	c, err := queue.NewCoordinator(o.store, run, opts.Parallel)
	if err != nil {
		return nil, err
	}
	if err := o.listen(probeSocket); err != nil {
		return nil, err
	}
	return &Server{owner: o, queue: c, limit: opts.Limit, done: make(chan struct{}), stop: make(chan struct{}), fault: make(chan error, 1)}, nil
}

func (s *Server) Run(ctx context.Context) (err error) {
	if !s.state.CompareAndSwap(0, 1) {
		return ErrStarted
	}
	defer func() {
		err = errors.Join(err, s.owner.close())
		s.err = err
		close(s.done)
	}()
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- s.queue.Run(ctx) }()
	end := make(chan struct{})
	accepted := make(chan error, 1)
	var clients sync.WaitGroup
	listener := s.owner.listener
	go func() { accepted <- s.accept(listener, end, &clients) }()
	select {
	case err = <-result:
	case err = <-accepted:
		cancel()
		err = errors.Join(err, <-result)
		accepted = nil
	case err = <-s.fault:
		cancel()
		err = errors.Join(err, <-result)
	case <-s.stop:
		cancel()
		err = <-result
		if err == context.Canceled && parent.Err() == nil {
			err = nil
		}
	}
	close(end)
	if cause := listener.Close(); !errors.Is(cause, net.ErrClosed) {
		err = errors.Join(err, cause)
	}
	s.owner.listener = nil
	if accepted != nil {
		<-accepted
	}
	clients.Wait()
	return err
}

func (s *Server) Close() error {
	if s.state.CompareAndSwap(0, 2) {
		s.err = s.owner.close()
		close(s.done)
	} else {
		if s.state.Load() == 1 {
			s.once.Do(func() { close(s.stop) })
		}
		<-s.done
	}
	return s.err
}

func (s *Server) Done() <-chan struct{} {
	return s.done
}

func (s *Server) accept(listener *net.UnixListener, end <-chan struct{}, clients *sync.WaitGroup) error {
	slots := make(chan struct{}, MaxClients)
	for {
		select {
		case slots <- struct{}{}:
		case <-end:
			return nil
		}
		conn, err := listener.AcceptUnix()
		if err != nil {
			<-slots
			return err
		}
		clients.Go(func() {
			defer func() { <-slots }()
			defer conn.Close()
			s.serve(conn)
		})
	}
}

func (s *Server) serve(conn *net.UnixConn) {
	if checkPeer(conn, uint32(os.Geteuid())) != nil {
		return
	}
	if err := s.owner.check(); err != nil {
		select {
		case s.fault <- err:
		default:
		}
		return
	}
	if conn.SetReadDeadline(time.Now().Add(Timeout)) != nil {
		return
	}
	data, err := readFrame(conn, MaxRequestSize)
	var result any
	if err == nil {
		var cmd command
		cmd, err = decodeCommand(data, s.owner.paths.State)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), Timeout)
			result, err = s.dispatch(ctx, cmd)
			cancel()
		}
	}
	if conn.SetWriteDeadline(time.Now().Add(Timeout)) == nil {
		writeFrame(conn, encodeReply(result, err))
	}
}

func (s *Server) dispatch(ctx context.Context, cmd command) (any, error) {
	switch cmd.op {
	case "ping", "list":
		view, err := s.queue.List(ctx)
		if err != nil {
			return nil, err
		}
		status := Status{Status: view.Daemon, LimitBPS: s.limit}
		if cmd.op == "ping" {
			return status, nil
		}
		return List{Daemon: status, Jobs: view.Jobs}, nil
	case "stop":
		status, err := s.queue.Stop(ctx)
		return Status{Status: status, LimitBPS: s.limit}, err
	case "set_parallel":
		status, err := s.queue.SetParallel(ctx, cmd.parallel)
		return Status{Status: status, LimitBPS: s.limit}, err
	case "add":
		return s.queue.Add(ctx, cmd.id, cmd.req)
	case "pause":
		return s.queue.Pause(ctx, cmd.id)
	case "resume":
		return s.queue.Resume(ctx, cmd.id)
	case "cancel":
		return s.queue.Cancel(ctx, cmd.id)
	case "forget":
		forgotten, err := s.queue.Forget(ctx, cmd.id)
		return Forgotten{ID: cmd.id, Forgotten: forgotten}, err
	default:
		return nil, ErrProtocol
	}
}
