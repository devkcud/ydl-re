package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

const LockName = "daemon.lock"
const SocketName = "control.sock"
const Timeout = 5 * time.Second

var ErrUnsupported = errors.New("daemon operations require Linux")
var ErrPaths = errors.New("invalid daemon state or runtime path")
var ErrAccess = errors.New("daemon artifacts must be private and owned by the effective user")
var ErrChanged = errors.New("daemon ownership artifacts changed")
var ErrLocked = errors.New("daemon endpoint is already owned")
var ErrEndpoint = errors.New("existing daemon endpoint cannot be replaced safely")
var ErrPeer = errors.New("daemon peer identity does not match the effective user")

type Paths struct {
	State    string
	Runtime  string
	Fallback bool
}

func Resolve() (Paths, error) {
	return resolvePaths(os.Getenv("HOME"), os.Getenv("XDG_STATE_HOME"), os.Getenv("XDG_RUNTIME_DIR"))
}

func (p Paths) Socket() string {
	return filepath.Join(p.Runtime, SocketName)
}
