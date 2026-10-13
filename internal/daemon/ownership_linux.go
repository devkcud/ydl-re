package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"yoinkdl/internal/queue"
)

const pathOnly = 0x200000

type owner struct {
	paths    Paths
	store    *queue.Store
	root     *os.Root
	dir      *os.File
	lock     *os.File
	listener *net.UnixListener
	socket   os.FileInfo
}

func acquire(p Paths) (_ *owner, err error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if !p.Fallback {
		info, err := os.Lstat(filepath.Dir(p.Runtime))
		if err != nil || !privateDirectory(info) || info.Mode().Perm() != 0o700 {
			return nil, ErrAccess
		}
	}
	if err := os.MkdirAll(filepath.Dir(p.State), 0o700); err != nil {
		return nil, err
	}
	o := &owner{paths: p}
	defer func() {
		if err != nil {
			err = errors.Join(err, o.close())
		}
	}()
	o.store, err = queue.Open(p.State)
	if err != nil {
		return nil, err
	}
	if err := o.openRuntime(); err != nil {
		return nil, err
	}
	if err := o.openLock(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *owner) openRuntime() error {
	parent, err := os.OpenRoot(filepath.Dir(o.paths.Runtime))
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(o.paths.Runtime)
	if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := parent.Lstat(name)
	if err != nil || !privateDirectory(info) {
		return ErrAccess
	}
	o.root, err = parent.OpenRoot(name)
	if err != nil {
		return err
	}
	o.dir, err = o.root.Open(".")
	if err != nil {
		return err
	}
	opened, err := o.dir.Stat()
	if err != nil || !privateDirectory(opened) || !os.SameFile(info, opened) {
		return ErrChanged
	}
	if err := o.checkDirectory(); err != nil {
		return err
	}
	base, err := parent.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(base.Sync(), base.Close())
}

func (o *owner) openLock() error {
	for range 2 {
		info, err := o.root.Lstat(LockName)
		flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
		if errors.Is(err, os.ErrNotExist) {
			flags |= os.O_CREATE | os.O_EXCL
		} else if err != nil {
			return err
		} else if !privateLock(info) {
			return ErrAccess
		}
		o.lock, err = o.root.OpenFile(LockName, flags, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		opened, err := o.lock.Stat()
		if err != nil || !privateLock(opened) || info != nil && !os.SameFile(info, opened) {
			return ErrChanged
		}
		if err := syscall.Flock(int(o.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return ErrLocked
			}
			return err
		}
		return errors.Join(o.check(), o.lock.Sync(), o.dir.Sync())
	}
	return ErrChanged
}

func (o *owner) checkDirectory() error {
	info, err := os.Lstat(o.paths.Runtime)
	if err != nil || !privateDirectory(info) {
		return ErrChanged
	}
	opened, err := o.dir.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return ErrChanged
	}
	return nil
}

func (o *owner) check() error {
	if err := o.checkDirectory(); err != nil {
		return err
	}
	opened, err := o.lock.Stat()
	if err != nil || !privateLock(opened) {
		return ErrChanged
	}
	info, err := o.root.Lstat(LockName)
	if err != nil || !privateLock(info) || !os.SameFile(info, opened) {
		return ErrChanged
	}
	return nil
}

func (o *owner) listen(probe func(string) error) error {
	if err := o.check(); err != nil {
		return err
	}
	info, err := o.root.Lstat(SocketName)
	if err == nil {
		if !ownedSocket(info) || info.Mode().Perm()&0o077 != 0 {
			return ErrEndpoint
		}
		if err := probe(o.anchoredSocket()); !errors.Is(err, syscall.ECONNREFUSED) {
			return ErrEndpoint
		}
		if err := o.check(); err != nil {
			return err
		}
		if err := o.removeSocket(info); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	o.listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: o.anchoredSocket(), Net: "unix"})
	if err != nil {
		return err
	}
	o.listener.SetUnlinkOnClose(false)
	file, err := o.root.OpenFile(SocketName, pathOnly|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	o.socket, err = file.Stat()
	if err != nil || !ownedSocket(o.socket) {
		o.socket = nil
		return ErrChanged
	}
	if err := os.Chmod(fmt.Sprintf("/proc/self/fd/%d", file.Fd()), 0o600); err != nil {
		return err
	}
	current, err := o.root.Lstat(SocketName)
	if err != nil || !ownedSocket(current) || current.Mode().Perm() != 0o600 || !os.SameFile(o.socket, current) {
		return ErrChanged
	}
	return errors.Join(o.check(), o.dir.Sync())
}

func (o *owner) anchoredSocket() string {
	return fmt.Sprintf("/proc/self/fd/%d/%s", o.dir.Fd(), SocketName)
}

func (o *owner) removeSocket(info os.FileInfo) error {
	current, err := o.root.Lstat(SocketName)
	if err != nil || !ownedSocket(current) || !os.SameFile(info, current) {
		return ErrChanged
	}
	return o.root.Remove(SocketName)
}

func (o *owner) close() error {
	var err error
	if o.listener != nil {
		err = o.listener.Close()
		o.listener = nil
	}
	if o.socket != nil {
		err = errors.Join(err, o.removeSocket(o.socket), o.dir.Sync())
		o.socket = nil
	}
	for _, file := range []*os.File{o.lock, o.dir} {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
	}
	o.lock, o.dir = nil, nil
	if o.root != nil {
		err = errors.Join(err, o.root.Close())
		o.root = nil
	}
	if o.store != nil {
		err = errors.Join(err, o.store.Close())
		o.store = nil
	}
	return err
}

func privateDirectory(info os.FileInfo) bool {
	return owned(info) && info.IsDir() && info.Mode().Perm()&0o077 == 0
}

func privateLock(info os.FileInfo) bool {
	return owned(info) && info.Mode().IsRegular() && info.Mode().Perm()&0o077 == 0 && info.Sys().(*syscall.Stat_t).Nlink == 1
}

func ownedSocket(info os.FileInfo) bool {
	return owned(info) && info.Mode().Type() == os.ModeSocket && info.Sys().(*syscall.Stat_t).Nlink == 1
}

func owned(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func probeSocket(path string) error {
	conn, err := net.DialTimeout("unix", path, Timeout)
	if err == nil {
		conn.Close()
	}
	return err
}

func checkPeer(conn *net.UnixConn, uid uint32) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return ErrPeer
	}
	var cred *syscall.Ucred
	var cause error
	err = raw.Control(func(fd uintptr) {
		cred, cause = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil || cause != nil || cred == nil || cred.Uid != uid {
		return ErrPeer
	}
	return nil
}
