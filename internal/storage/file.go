package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const PartSuffix = ".yoinkdl.part"
const ManifestSuffix = ".yoinkdl.json"

type File struct {
	root      *os.Root
	file      *os.File
	path      string
	name      string
	part      string
	overwrite bool
	written   int64
	partial   bool
	published bool
}

func Create(path string, overwrite bool) (*File, error) {
	if path == "" || strings.HasSuffix(path, PartSuffix) || strings.HasSuffix(path, ManifestSuffix) {
		return nil, errors.New("output must name a file outside the reserved .yoinkdl.part and .yoinkdl.json suffixes")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, errors.New("downloads currently require Linux or macOS filesystem operations")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve output: %w", err)
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open output directory: %w", err)
	}
	f := &File{root: root, path: path, name: filepath.Base(path), overwrite: overwrite}
	f.part = f.name + PartSuffix
	if err := f.checkDestination(); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	if _, err := root.Lstat(f.name + ManifestSuffix); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			err = errors.New("existing transfer manifest; choose another output path")
		}
		return nil, errors.Join(err, root.Close())
	}
	f.file, err = root.OpenFile(f.part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create partial file (existing partials are preserved): %w", err), root.Close())
	}
	f.partial = true
	return f, nil
}

func (f *File) Path() string {
	return f.path
}

func (f *File) Partial() string {
	if f.partial {
		return f.path + PartSuffix
	}
	return ""
}

func (f *File) Published() bool {
	return f.published
}

func (f *File) Write(data []byte) (int, error) {
	if f.file == nil {
		return 0, os.ErrClosed
	}
	n, err := f.file.Write(data)
	f.written += int64(n)
	return n, err
}

func (f *File) Finalize() error {
	if f.file == nil {
		return os.ErrClosed
	}
	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("sync partial file: %w", err)
	}
	err := f.file.Close()
	f.file = nil
	if err != nil {
		return fmt.Errorf("close partial file: %w", err)
	}
	if f.overwrite {
		if err := f.checkDestination(); err != nil {
			return err
		}
		err = f.root.Rename(f.part, f.name)
	} else {
		err = f.root.Link(f.part, f.name)
	}
	if err != nil {
		return fmt.Errorf("publish output: %w", err)
	}
	f.published = true
	if f.overwrite {
		f.partial = false
	}
	if err := f.syncDirectory(); err != nil {
		return fmt.Errorf("output published but directory sync failed: %w", err)
	}
	if f.partial {
		if err := f.root.Remove(f.part); err != nil {
			return fmt.Errorf("output published but partial cleanup failed: %w", err)
		}
		f.partial = false
		if err := f.syncDirectory(); err != nil {
			return fmt.Errorf("output published but cleanup sync failed: %w", err)
		}
	}
	return nil
}

func (f *File) Close() error {
	if f.root == nil {
		return nil
	}
	var err error
	if f.file != nil {
		if f.written > 0 {
			err = f.file.Sync()
		}
		err = errors.Join(err, f.file.Close())
		f.file = nil
	}
	if f.partial && f.written == 0 && !f.published {
		if removeErr := f.root.Remove(f.part); removeErr != nil {
			err = errors.Join(err, removeErr)
		} else {
			f.partial = false
		}
	}
	err = errors.Join(err, f.root.Close())
	f.root = nil
	return err
}

func (f *File) checkDestination() error {
	info, err := f.root.Lstat(f.name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("output must be a regular file, not a directory, symlink, or special file")
	}
	if !f.overwrite {
		return errors.New("output already exists; use --overwrite to replace it")
	}
	return nil
}

func (f *File) syncDirectory() error {
	dir, err := f.root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
