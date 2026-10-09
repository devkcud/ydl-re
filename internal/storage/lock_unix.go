//go:build linux || darwin

package storage

import (
	"errors"
	"os"
	"syscall"
)

func openRegular(root *os.Root, name string, flags int) (*os.File, error) {
	file, err := root.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("transfer artifact must be a regular file")
	}

	return file, nil
}

func acquireLock(root *os.Root, name string) (*os.File, error) {
	file, err := openRegular(root, name, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("transfer is in use or filesystem locking is unavailable")
	}

	return file, nil
}
