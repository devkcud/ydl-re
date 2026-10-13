package queue

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStorePreRenameFailuresPreserveCommittedState(t *testing.T) {
	failure := errors.New("injected IO failure")
	for _, phase := range []string{"partial write", "short write", "file sync", "rename"} {
		t.Run(phase, func(t *testing.T) {
			dir := privateDir(t)
			s := testStore(t, dir)
			before := testSnapshot(t, dir, 1)
			must(t, s.Save(before))
			path := filepath.Join(dir, SnapshotName)
			data := readFile(t, path)
			stale := filepath.Join(dir, TempPrefix+"stale")
			writeFile(t, stale, []byte("preserve"))
			wantErr := failure
			switch phase {
			case "partial write":
				s.ops.write = func(f *os.File, data []byte) (int, error) {
					n, err := f.Write(data[:len(data)/2])
					return n, errors.Join(err, failure)
				}
			case "short write":
				s.ops.write = func(f *os.File, data []byte) (int, error) { return f.Write(data[:len(data)/2]) }
				wantErr = io.ErrShortWrite
			case "file sync":
				s.ops.syncFile = func(*os.File) error { return failure }
			case "rename":
				s.ops.rename = func(*os.Root, string, string) error { return failure }
			}

			next := before.clone()
			next.Parallel = 2
			if err := s.Save(next); !errors.Is(err, wantErr) || errors.Is(err, ErrUncertain) {
				t.Fatalf("pre-commit error was misclassified: %v", err)
			}
			assertStore(t, s, before)
			if !bytes.Equal(data, readFile(t, path)) || string(readFile(t, stale)) != "preserve" {
				t.Fatal("failed commit modified existing state")
			}
			entries, err := os.ReadDir(dir)
			must(t, err)
			if len(entries) != 3 {
				t.Fatal("failed write leaked its owned temporary file")
			}

			s.ops = defaultStoreOps()
			must(t, s.Save(next))
			assertStore(t, s, next)
		})
	}
}

func TestStoreUncertainCommitsRequireReopen(t *testing.T) {
	failure := errors.New("injected IO failure")
	for _, phase := range []string{"rename result", "directory sync"} {
		t.Run(phase, func(t *testing.T) {
			dir := privateDir(t)
			s := testStore(t, dir)
			if phase == "rename result" {
				s.ops.rename = func(root *os.Root, from, to string) error {
					return errors.Join(root.Rename(from, to), failure)
				}
			} else {
				s.ops.syncDir = func(*os.File) error { return failure }
			}

			next := testSnapshot(t, dir, 1)
			if err := s.Save(next); !errors.Is(err, ErrUncertain) || !errors.Is(err, failure) {
				t.Fatalf("ambiguous commit was not reported: %v", err)
			}
			if _, err := s.Snapshot(); !errors.Is(err, ErrUncertain) {
				t.Fatal("uncertain store served a stale authoritative snapshot")
			}
			if err := s.Save(NewSnapshot()); !errors.Is(err, ErrUncertain) {
				t.Fatal("uncertain store allowed rollback")
			}
			other, err := Open(dir)
			if other != nil || !errors.Is(err, ErrLocked) {
				t.Fatal("faulted store released its writer lock early")
			}
			got, err := decodeSnapshot(readFile(t, filepath.Join(dir, SnapshotName)))
			if err != nil || !reflect.DeepEqual(got, next) {
				t.Fatal("failed acknowledgment overwrote the possibly committed snapshot")
			}
			must(t, s.Close())
			assertStore(t, testStore(t, dir), next)
		})
	}
}

func TestStoreInitialCommitFailureReleasesOwnership(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	ops := defaultStoreOps()
	ops.syncDir = func(*os.File) error { return errors.New("injected sync failure") }
	s, err := openStore(dir, ops)
	if s != nil || !errors.Is(err, ErrUncertain) {
		t.Fatalf("initial commit failure was accepted: %v", err)
	}
	assertStore(t, testStore(t, dir), NewSnapshot())
}

func TestStoreExposesStateOnlyAfterSync(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	var phases []string
	next := testSnapshot(t, dir, 1)
	s.ops.write = func(f *os.File, data []byte) (int, error) {
		phases = append(phases, "write")
		return f.Write(data)
	}
	s.ops.syncFile = func(f *os.File) error {
		phases = append(phases, "file sync")
		return f.Sync()
	}
	s.ops.rename = func(root *os.Root, from, to string) error {
		phases = append(phases, "rename")
		return root.Rename(from, to)
	}
	s.ops.syncDir = func(f *os.File) error {
		phases = append(phases, "directory sync")
		if !reflect.DeepEqual(s.state, NewSnapshot()) {
			t.Fatal("candidate was exposed before durable commit")
		}
		got, err := decodeSnapshot(readFile(t, filepath.Join(dir, SnapshotName)))
		if err != nil || !reflect.DeepEqual(got, next) {
			t.Fatal("directory sync ran before snapshot replacement")
		}
		return f.Sync()
	}
	must(t, s.Save(next))
	if !reflect.DeepEqual(phases, []string{"write", "file sync", "rename", "directory sync"}) {
		t.Fatalf("commit order changed: %v", phases)
	}
	assertStore(t, s, next)
}

func TestStorePreservesReplacedTemporaryArtifacts(t *testing.T) {
	for _, kind := range []string{"replacement", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := privateDir(t)
			s := testStore(t, dir)
			var name string
			s.ops.syncFile = func(f *os.File) error {
				must(t, f.Sync())
				name = filepath.Base(f.Name())
				if kind == "replacement" {
					must(t, s.root.Rename(name, "moved"))
					writeFile(t, filepath.Join(dir, name), []byte("foreign"))
				} else {
					must(t, os.Link(filepath.Join(dir, name), filepath.Join(dir, "alias")))
				}
				return nil
			}
			next := NewSnapshot()
			next.Parallel = 2
			if err := s.Save(next); err == nil {
				t.Fatal("changed temporary artifact was published")
			}
			assertStore(t, s, NewSnapshot())
			data := readFile(t, filepath.Join(dir, name))
			if kind == "replacement" && string(data) != "foreign" {
				t.Fatal("replacement temporary artifact was deleted or changed")
			}
			got, err := decodeSnapshot(readFile(t, filepath.Join(dir, SnapshotName)))
			if err != nil || !reflect.DeepEqual(got, NewSnapshot()) {
				t.Fatal("canonical state changed after temporary-file refusal")
			}
		})
	}
}

func TestStoreRechecksOwnershipBeforeRename(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	path := filepath.Join(dir, SnapshotName)
	changed := strings.ReplaceAll(string(readFile(t, path)), `"parallel":1`, `"parallel":2`)
	s.ops.syncFile = func(f *os.File) error {
		must(t, f.Sync())
		writeFile(t, path, []byte(changed))
		return nil
	}
	if err := s.Save(NewSnapshot()); !errors.Is(err, ErrChanged) || string(readFile(t, path)) != changed {
		t.Fatal("late canonical change was overwritten")
	}
}
