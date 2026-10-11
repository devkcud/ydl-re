//go:build !linux && !darwin

package cli

import "os"

func terminalSize(file *os.File) (int, bool) {
	return 0, false
}
