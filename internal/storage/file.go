package storage

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

const PartSuffix = ".yoinkdl.part"
const ManifestSuffix = ".yoinkdl.json"
const LockSuffix = ".yoinkdl.lock"
const TempPrefix = ".yoinkdl.tmp-"

type File struct {
	root      *os.Root
	file      *os.File
	lock      *os.File
	info      os.FileInfo
	path      string
	name      string
	part      string
	overwrite bool
	written   int64
	partial   bool
	published bool
	fresh     bool
	active    bool
	fromFinal bool
	manifest  *Manifest
	digest    hash.Hash
	last      time.Time
}

func Reserved(path string) bool {
	name := filepath.Base(path)
	return strings.HasSuffix(name, PartSuffix) || strings.HasSuffix(name, ManifestSuffix) ||
		strings.HasSuffix(name, LockSuffix) || strings.Contains(name, TempPrefix)
}

func newFile(path string, overwrite bool) (*File, error) {
	if path == "" || Reserved(path) || !utf8.ValidString(path) {
		return nil, errors.New("output must name a file outside the reserved transfer artifact names")
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

	f := &File{
		root:      root,
		path:      path,
		name:      filepath.Base(path),
		overwrite: overwrite,
	}
	f.part = f.name + PartSuffix
	f.lock, err = acquireLock(root, f.name+LockSuffix)

	if err != nil {
		return nil, errors.Join(err, root.Close())
	}

	return f, nil
}

func Create(path string, overwrite bool) (*File, error) {
	f, err := newFile(path, overwrite)
	if err != nil {
		return nil, err
	}

	if err := f.create(); err != nil {
		return nil, errors.Join(err, f.Close())
	}

	return f, nil
}

func Open(ctx context.Context, path string, overwrite, requireResume bool) (*File, error) {
	f, err := newFile(path, overwrite)
	if err != nil {
		return nil, err
	}

	_, err = f.root.Lstat(f.name + ManifestSuffix)

	switch {
	case err == nil:
		err = f.restore(ctx)
	case !errors.Is(err, os.ErrNotExist):
	case requireResume:
		err = errors.New("no saved transfer manifest to resume")
	default:
		err = f.create()
	}

	if err != nil {
		return nil, errors.Join(err, f.Close())
	}

	return f, nil
}

func (f *File) create() error {
	if err := f.checkDestination(); err != nil {
		return err
	}

	if _, err := f.root.Lstat(f.name + ManifestSuffix); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			err = errors.New("existing transfer manifest; choose another output path")
		}

		return err
	}

	var err error
	f.file, err = f.root.OpenFile(f.part, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create partial file (existing partials are preserved): %w", err)
	}

	f.partial = true
	f.fresh = true
	f.info, err = f.file.Stat()
	return err
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

func (f *File) Metadata() (Manifest, bool) {
	if f.manifest == nil {
		return Manifest{}, false
	}

	return f.manifest.clone(), true
}

func (f *File) Write(data []byte) (int, error) {
	if f.file == nil {
		return 0, os.ErrClosed
	}

	if f.manifest != nil {
		if !f.active || f.manifest.Version != ManifestVersion || f.manifest.State == StateReady {
			return 0, errors.New("transfer is not ready to receive bytes")
		}

		if f.manifest.Length >= 0 && int64(len(data)) > f.manifest.Length-f.written {
			return 0, errors.New("write exceeds the expected representation length")
		}
	}

	n, err := f.file.Write(data)
	f.written += int64(n)

	if f.digest != nil {
		f.digest.Write(data[:n])
	}

	if err == nil {
		err = f.Checkpoint(false)
	}

	return n, err
}

func (f *File) Finalize() error {
	if f.file == nil {
		return os.ErrClosed
	}

	if err := f.prepare(); err != nil {
		return err
	}

	if err := f.checkIdentity(); err != nil {
		return err
	}

	err := f.file.Close()
	f.file = nil
	if err != nil {
		return fmt.Errorf("close partial file: %w", err)
	}

	info, statErr := f.root.Lstat(f.name)
	linked := f.manifest != nil && statErr == nil && os.SameFile(info, f.info)

	switch {
	case f.fromFinal || linked:
	case f.overwrite:
		if err := f.checkDestination(); err != nil {
			return err
		}

		err = f.root.Rename(f.part, f.name)
		if err == nil {
			f.partial = false
		}
	default:
		err = f.root.Link(f.part, f.name)
	}

	if err != nil {
		return fmt.Errorf("publish output: %w", err)
	}

	f.published = true
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

	if f.manifest != nil {
		if err := f.root.Remove(f.name + ManifestSuffix); err != nil {
			return fmt.Errorf("output published but manifest cleanup failed: %w", err)
		}

		if err := f.syncDirectory(); err != nil {
			return fmt.Errorf("output published but manifest cleanup sync failed: %w", err)
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
		err = f.Checkpoint(true)

		if f.written > 0 {
			err = errors.Join(err, f.file.Sync())
		}

		err = errors.Join(err, f.file.Close())
		f.file = nil
	}

	if f.fresh && f.partial && f.written == 0 && !f.published &&
		(f.manifest == nil || (f.manifest.Version == ManifestVersion && f.manifest.State != StateReady)) {
		if identityErr := f.checkIdentity(); identityErr != nil {
			err = errors.Join(err, identityErr)
		} else if removeErr := f.root.Remove(f.part); removeErr != nil {
			err = errors.Join(err, removeErr)
		} else {
			f.partial = false

			if f.manifest != nil {
				err = errors.Join(err, f.root.Remove(f.name+ManifestSuffix))
			}

			err = errors.Join(err, f.syncDirectory())
		}
	}

	if f.lock != nil {
		err = errors.Join(err, f.lock.Close())
	}

	err = errors.Join(err, f.root.Close())
	f.root = nil
	return err
}

func (f *File) checkIdentity() error {
	name := f.part
	if f.fromFinal {
		name = f.name
	}

	info, err := f.root.Lstat(name)
	if err != nil || f.info == nil || !info.Mode().IsRegular() || !os.SameFile(info, f.info) {
		return errors.New("transfer file changed while it was owned")
	}

	return nil
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
