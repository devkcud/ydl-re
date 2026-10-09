//go:build !linux && !darwin

package storage

import (
	"errors"
	"os"
)

func openRegular(root *os.Root, name string, flags int) (*os.File, error) {
	return nil, errors.New("transfer storage currently requires Linux or macOS")
}

func acquireLock(root *os.Root, name string) (*os.File, error) {
	return nil, errors.New("transfer locking currently requires Linux or macOS")
}
