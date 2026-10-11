//go:build linux || darwin

package cli

import (
	"errors"
	"os"
	"syscall"
	"time"
)

func openOutput(file *os.File) (*os.File, func() error, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeSocket) == 0 {
		return file, nil, nil
	}

	raw, err := file.SyscallConn()
	if err != nil {
		return nil, nil, err
	}

	fd := -1
	var flags uintptr
	var callErr error
	err = raw.Control(func(source uintptr) {
		var errno syscall.Errno
		flags, _, errno = syscall.Syscall(syscall.SYS_FCNTL, source, syscall.F_GETFL, 0)
		if errno != 0 {
			callErr = errno
			return
		}

		fd, callErr = syscall.Dup(int(source))
		if callErr != nil {
			return
		}

		syscall.CloseOnExec(fd)
		callErr = syscall.SetNonblock(fd, true)
	})
	if err = errors.Join(err, callErr); err != nil {
		if fd >= 0 {
			err = errors.Join(err, syscall.Close(fd))
		}
		return nil, nil, err
	}

	out := os.NewFile(uintptr(fd), file.Name())
	close := func() error {
		var err error
		if flags&syscall.O_NONBLOCK == 0 {
			err = syscall.SetNonblock(fd, false)
		}
		return errors.Join(err, out.Close())
	}
	if err := out.SetWriteDeadline(time.Time{}); err != nil {
		return nil, nil, errors.Join(err, close())
	}

	return out, close, nil
}
