package daemon

import (
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"yoinkdl/internal/queue"
)

func resolvePaths(home, state, runtime string) (Paths, error) {
	if !absolute(state) {
		if !absolute(home) {
			return Paths{}, ErrPaths
		}
		state = filepath.Join(home, ".local", "state")
	}
	p := Paths{State: filepath.Join(state, "yoinkdl")}
	if absolute(runtime) {
		p.Runtime = filepath.Join(runtime, "yoinkdl")
	} else {
		p.Runtime = filepath.Join(p.State, "run")
		p.Fallback = true
	}
	return p, p.validate()
}

func (p Paths) validate() error {
	if !absolute(p.State) || !absolute(p.Runtime) || filepath.Clean(p.State) != p.State ||
		filepath.Clean(p.Runtime) != p.Runtime || len(p.State) > queue.MaxPathSize ||
		len(p.Socket()) >= len(syscall.RawSockaddrUnix{}.Path) ||
		p.Fallback && p.Runtime != filepath.Join(p.State, "run") {
		return ErrPaths
	}
	return nil
}

func absolute(path string) bool {
	return filepath.IsAbs(path) && utf8.ValidString(path) && !strings.ContainsRune(path, 0)
}
