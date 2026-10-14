package daemon

import "errors"

var ErrStarted = errors.New("daemon can run only once")

type Options struct {
	Paths    Paths
	Parallel int
	Limit    int64
}
