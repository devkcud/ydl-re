//go:build !linux && !darwin

package cli

import "os"

func openOutput(file *os.File) (*os.File, func() error, error) {
	return file, nil, nil
}
