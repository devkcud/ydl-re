package queue

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func openDirectory(path string) (_ *os.Root, _ *os.File, err error) {
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, nil, err
	}
	defer parent.Close()

	name := filepath.Base(path)
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, nil, err
		}

		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, nil, err
	}
	if err := checkDirectory(info); err != nil {
		return nil, nil, err
	}

	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, root.Close())
		}
	}()

	dir, err := root.Open(".")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, dir.Close())
		}
	}()

	opened, err := dir.Stat()
	if err != nil {
		return nil, nil, err
	}
	if err := checkDirectory(opened); err != nil {
		return nil, nil, err
	}

	current, err := parent.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if err := checkDirectory(current); err != nil {
		return nil, nil, err
	}
	if !os.SameFile(info, opened) || !os.SameFile(opened, current) {
		return nil, nil, ErrChanged
	}

	base, err := parent.Open(".")
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(base.Sync(), base.Close()); err != nil {
		return nil, nil, err
	}

	return root, dir, nil
}

func openPrivate(root *os.Root, name string, flags int, create bool) (*os.File, error) {
	for range 2 {
		info, err := root.Lstat(name)
		mode := flags | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
		if errors.Is(err, os.ErrNotExist) && create {
			mode |= os.O_CREATE | os.O_EXCL
		} else if err != nil {
			return nil, err
		} else if err := checkFile(info); err != nil {
			return nil, err
		}

		file, err := root.OpenFile(name, mode, 0o600)
		if errors.Is(err, os.ErrExist) && create {
			continue
		}
		if err != nil {
			return nil, err
		}

		opened, err := file.Stat()
		if err == nil {
			err = checkFile(opened)
		}
		if err == nil && info != nil && !os.SameFile(info, opened) {
			err = ErrChanged
		}
		if err == nil {
			err = checkPath(root, name, opened)
		}
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}

		return file, nil
	}

	return nil, ErrChanged
}

func checkDirectory(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o077 != 0 {
		return errors.New("queue directory must be private and owned by the effective user")
	}

	return nil
}

func checkFile(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || stat.Uid != uint32(os.Geteuid()) ||
		stat.Nlink != 1 || info.Mode().Perm()&0o077 != 0 {
		return errors.New("queue artifact must be private, single-linked, regular, and owned by the effective user")
	}

	return nil
}

func checkPath(root *os.Root, name string, info os.FileInfo) error {
	current, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if err := checkFile(current); err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return ErrChanged
	}

	return nil
}

func lockFile(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrLocked
	}

	return err
}
