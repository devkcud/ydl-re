//go:build linux || darwin

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

func terminalSize(file *os.File) (int, bool) {
	raw, err := file.SyscallConn()
	if err != nil {
		return 0, false
	}

	var size [4]uint16
	var errno syscall.Errno
	err = raw.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	})
	return int(size[1]), err == nil && errno == 0
}
