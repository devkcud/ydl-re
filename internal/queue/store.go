package queue

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const SnapshotName = "queue.json"
const LockName = "queue.lock"
const TempPrefix = ".queue.tmp-"

var ErrLocked = errors.New("queue is already in use")
var ErrChanged = errors.New("queue store artifacts changed; close and reopen the store")
var ErrUncertain = errors.New("queue commit durability is uncertain; close and reopen the store")
var ErrUnsupported = errors.New("queue storage requires Linux")

type Store struct {
	mu    sync.Mutex
	root  *os.Root
	dir   *os.File
	lock  *os.File
	info  os.FileInfo
	sum   [sha256.Size]byte
	state Snapshot
	fault error
	ops   storeOps
}

type storeOps struct {
	write    func(*os.File, []byte) (int, error)
	syncFile func(*os.File) error
	rename   func(*os.Root, string, string) error
	syncDir  func(*os.File) error
}

func defaultStoreOps() storeOps {
	return storeOps{write: (*os.File).Write, syncFile: (*os.File).Sync, rename: (*os.Root).Rename, syncDir: (*os.File).Sync}
}

func Open(path string) (*Store, error) {
	return openStore(path, defaultStoreOps())
}

func openStore(path string, ops storeOps) (_ *Store, err error) {
	if !utf8.ValidString(path) || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("queue state directory must be an absolute clean path")
	}

	root, dir, err := openDirectory(path)
	if err != nil {
		return nil, err
	}

	s := &Store{root: root, dir: dir, state: NewSnapshot(), ops: ops}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.Close())
		}
	}()

	s.lock, err = openPrivate(root, LockName, os.O_RDWR, true)
	if err != nil {
		return nil, fmt.Errorf("open queue lock: %w", err)
	}

	if err := lockFile(s.lock); err != nil {
		return nil, err
	}

	if err := s.checkOwnership(); err != nil {
		return nil, err
	}

	data, info, err := s.readSnapshot()
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := s.checkTemps(); err != nil {
			return nil, err
		}

		if err := s.Save(NewSnapshot()); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		s.state, err = decodeSnapshot(data)
		if err != nil {
			return nil, err
		}

		s.info = info
		s.sum = sha256.Sum256(data)
	}

	return s, nil
}

func (s *Store) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.available(); err != nil {
		return Snapshot{}, err
	}

	return s.state.clone(), nil
}

func (s *Store) Save(next Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.available(); err != nil {
		return err
	}

	next = next.clone()
	data, err := encodeSnapshot(next)
	if err != nil {
		return err
	}

	if err := validateUpdate(s.state, next); err != nil {
		return err
	}

	if err := s.verify(); err != nil {
		s.fault = errors.Join(ErrChanged, err)
		return s.fault
	}

	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return fmt.Errorf("create queue checkpoint name: %w", err)
	}

	name := TempPrefix + hex.EncodeToString(token[:])
	temp, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create queue checkpoint: %w", err)
	}
	defer temp.Close()

	created, err := temp.Stat()
	if err != nil {
		return fmt.Errorf("stat queue checkpoint: %w", err)
	}
	defer s.removeTemp(name, created)

	if err := checkFile(created); err != nil {
		return err
	}

	n, err := s.ops.write(temp, data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write queue checkpoint: %w", err)
	}

	if err := s.ops.syncFile(temp); err != nil {
		return fmt.Errorf("sync queue checkpoint: %w", err)
	}

	info, err := temp.Stat()
	if err != nil {
		return fmt.Errorf("stat written queue checkpoint: %w", err)
	}
	if err := checkFile(info); err != nil {
		return err
	}
	if info.Size() != int64(len(data)) {
		return errors.New("queue checkpoint size changed")
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("close queue checkpoint: %w", err)
	}

	if err := checkPath(s.root, name, info); err != nil {
		return err
	}

	if err := s.verify(); err != nil {
		s.fault = errors.Join(ErrChanged, err)
		return s.fault
	}

	if err := s.ops.rename(s.root, name, SnapshotName); err != nil {
		if verifyErr := s.verify(); verifyErr != nil {
			s.fault = errors.Join(ErrUncertain, err, verifyErr)
			return s.fault
		}

		return fmt.Errorf("replace queue checkpoint: %w", err)
	}

	if err := s.ops.syncDir(s.dir); err != nil {
		s.fault = errors.Join(ErrUncertain, err)
		return s.fault
	}

	s.state = next
	s.info = info
	s.sum = sha256.Sum256(data)
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.root == nil {
		return nil
	}

	var err error
	if s.dir != nil {
		err = s.dir.Close()
	}
	err = errors.Join(err, s.root.Close())
	if s.lock != nil {
		err = errors.Join(err, s.lock.Close())
	}

	s.root = nil
	s.dir = nil
	s.lock = nil
	return err
}

func (s *Store) available() error {
	if s.root == nil {
		return os.ErrClosed
	}

	return s.fault
}

func (s *Store) checkOwnership() error {
	info, err := s.dir.Stat()
	if err != nil {
		return err
	}
	if err := checkDirectory(info); err != nil {
		return err
	}

	info, err = s.lock.Stat()
	if err != nil {
		return err
	}
	if err := checkFile(info); err != nil {
		return err
	}

	return checkPath(s.root, LockName, info)
}

func (s *Store) verify() error {
	if err := s.checkOwnership(); err != nil {
		return err
	}

	data, info, err := s.readSnapshot()
	if s.info == nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if s.info == nil || !os.SameFile(s.info, info) || sha256.Sum256(data) != s.sum {
		return ErrChanged
	}

	return nil
}

func (s *Store) readSnapshot() (data []byte, info os.FileInfo, err error) {
	file, err := openPrivate(s.root, SnapshotName, os.O_RDONLY, false)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	before, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}

	data, err = io.ReadAll(io.LimitReader(file, MaxSnapshotSize+1))
	if err != nil || len(data) > MaxSnapshotSize {
		return nil, nil, errors.New("unreadable or oversized queue snapshot")
	}

	info, err = file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if err := checkFile(info); err != nil {
		return nil, nil, err
	}
	if info.Size() != int64(len(data)) || before.Size() != info.Size() || !before.ModTime().Equal(info.ModTime()) {
		return nil, nil, ErrChanged
	}
	if err := checkPath(s.root, SnapshotName, info); err != nil {
		return nil, nil, err
	}

	return data, info, nil
}

func (s *Store) checkTemps() error {
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()

	for {
		items, err := dir.ReadDir(64)
		for _, item := range items {
			if strings.HasPrefix(item.Name(), TempPrefix) {
				return errors.New("queue snapshot is missing beside temporary state; explicit recovery is required")
			}
		}

		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (s *Store) removeTemp(name string, info os.FileInfo) {
	if checkPath(s.root, name, info) == nil {
		s.root.Remove(name)
	}
}

func validateUpdate(before, after Snapshot) error {
	if after.NextOrder < before.NextOrder {
		return errors.New("queue order counter cannot move backward")
	}

	previous := make(map[string]Job, len(before.Jobs))
	for _, job := range before.Jobs {
		previous[job.ID] = job
	}

	for _, job := range after.Jobs {
		old, exists := previous[job.ID]
		if (!exists || job.Order != old.Order) && job.Order < before.NextOrder {
			return errors.New("queue cannot reuse an admission order")
		}
		if !exists {
			continue
		}

		a, b := old.Request, job.Request
		a.URL = nil
		b.URL = nil
		if old.InputHash != job.InputHash || a != b {
			return errors.New("retained queue request is immutable")
		}

		if job.Request.URL != nil && (old.Request.URL == nil || *old.Request.URL != *job.Request.URL) {
			return errors.New("retained queue source is immutable")
		}
	}

	return nil
}
