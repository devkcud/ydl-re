//go:build !linux

package daemon

func resolvePaths(home, state, runtime string) (Paths, error) {
	return Paths{}, ErrUnsupported
}
