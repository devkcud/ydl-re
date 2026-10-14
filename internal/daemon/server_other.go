//go:build !linux

package daemon

import "context"

type Server struct{}

func New(opts Options) (*Server, error) {
	return nil, ErrUnsupported
}

func (s *Server) Run(ctx context.Context) error {
	return ErrUnsupported
}

func (s *Server) Close() error {
	return ErrUnsupported
}

func (s *Server) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func Call(ctx context.Context, paths Paths, op string, args any) (Reply, error) {
	return Reply{}, ErrUnsupported
}
