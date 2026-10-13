package queue

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func TestStorePersistsPrivateOwnedSnapshots(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s := testStore(t, dir)
	assertStore(t, s, NewSnapshot())
	for name, mode := range map[string]os.FileMode{"": 0o700, LockName: 0o600, SnapshotName: 0o600} {
		info, err := os.Lstat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("unsafe initial artifact %q: %v", name, err)
		}
	}

	next := testSnapshot(t, dir, 2)
	next.Parallel = MaxParallel
	next.Jobs[1].State = Failed
	next.Jobs[1].Error = ptr("attempt failed")
	want := next.clone()
	if err := s.Save(next); err != nil {
		t.Fatal(err)
	}
	*next.Jobs[0].Request.URL = "changed"
	*next.Jobs[0].Published = true
	*next.Jobs[1].Error = "changed"
	assertStore(t, s, want)

	copy, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	*copy.Jobs[0].Request.URL = "changed"
	*copy.Jobs[0].Published = true
	*copy.Jobs[1].Error = "changed"
	assertStore(t, s, want)
	if !bytes.Contains(readFile(t, filepath.Join(dir, SnapshotName)), []byte("queue-private")) {
		t.Fatal("queued source was not retained")
	}

	for range 2 {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Snapshot(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("closed store returned a snapshot")
	}
	if err := s.Save(want); !errors.Is(err, os.ErrClosed) {
		t.Fatal("closed store accepted a mutation")
	}

	s = testStore(t, dir)
	assertStore(t, s, want)
	want.Parallel = 1
	for i := range want.Jobs {
		want.Jobs[i].State = Completed
		want.Jobs[i].Request.URL = nil
		want.Jobs[i].Published = ptr(true)
		want.Jobs[i].Error = nil
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readFile(t, filepath.Join(dir, SnapshotName)), []byte("queue-private")) {
		t.Fatal("terminal source survived in canonical state")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertStore(t, testStore(t, dir), want)
}

func TestStoreRejectsUnsafeDirectories(t *testing.T) {
	for _, kind := range []string{"relative", "unclean", "nul", "UTF8", "missing parent", "public", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			switch kind {
			case "relative":
				dir = "queue-state"
			case "unclean":
				dir += "/../state"
			case "nul":
				dir += "\x00"
			case "UTF8":
				dir += "\xff"
			case "missing parent":
				dir = filepath.Join(dir, "nested")
			case "public":
				must(t, os.Mkdir(dir, 0o700))
				must(t, os.Chmod(dir, 0o755))
			case "file":
				writeFile(t, dir, []byte("foreign"))
			case "symlink":
				must(t, os.Symlink(t.TempDir(), dir))
			}

			before, _ := os.Lstat(dir)
			s, err := Open(dir)
			if s != nil || err == nil {
				t.Fatal("unsafe directory was accepted")
			}
			if before != nil {
				after, err := os.Lstat(dir)
				if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
					t.Fatal("unsafe directory artifact was modified")
				}
			}
		})
	}
}

func TestStoreRejectsUnsafeFiles(t *testing.T) {
	for _, name := range []string{LockName, SnapshotName} {
		for _, kind := range []string{"public", "symlink", "hardlink", "directory", "fifo"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				dir := privateDir(t)
				path := filepath.Join(dir, name)
				target := filepath.Join(t.TempDir(), "foreign")
				writeFile(t, target, []byte("untouched"))
				switch kind {
				case "public":
					writeFile(t, path, []byte("foreign"))
					must(t, os.Chmod(path, 0o644))
				case "symlink":
					must(t, os.Symlink(target, path))
				case "hardlink":
					must(t, os.Link(target, path))
				case "directory":
					must(t, os.Mkdir(path, 0o700))
				case "fifo":
					must(t, syscall.Mkfifo(path, 0o600))
				}

				before, err := os.Lstat(path)
				must(t, err)
				s, err := Open(dir)
				if s != nil || err == nil {
					t.Fatal("unsafe queue artifact was accepted")
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
					t.Fatal("unsafe queue artifact was modified")
				}
				if string(readFile(t, target)) != "untouched" {
					t.Fatal("foreign target was modified")
				}
			})
		}
	}
}

func TestStoreRejectsForeignOwnership(t *testing.T) {
	dir := privateDir(t)
	path := filepath.Join(dir, "file")
	writeFile(t, path, nil)
	for _, path := range []string{dir, path} {
		info, err := os.Stat(path)
		must(t, err)
		stat := *info.Sys().(*syscall.Stat_t)
		stat.Uid++
		foreign := fileInfo{FileInfo: info, stat: &stat}
		if info.IsDir() {
			if checkDirectory(foreign) == nil {
				t.Fatal("foreign directory owner accepted")
			}
		} else if checkFile(foreign) == nil {
			t.Fatal("foreign file owner accepted")
		}
	}
}

func TestStorePreservesInvalidCanonicalState(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"version":1,"version":1,"parallel":1,"next_order":1,"jobs":[]}`),
		[]byte(`{"version":2,"parallel":1,"next_order":1,"jobs":[]}`),
		[]byte(`{"url":"https://example.invalid/?token=queue-private"}`),
		bytes.Repeat([]byte(" "), MaxSnapshotSize+1),
	} {
		dir := privateDir(t)
		path := filepath.Join(dir, SnapshotName)
		writeFile(t, path, data)
		for range 2 {
			s, err := Open(dir)
			assertPrivateError(t, err)
			if s != nil || !bytes.Equal(data, readFile(t, path)) {
				t.Fatal("invalid queue was reset or accepted")
			}
		}
	}
}

func TestStoreNeverAdoptsOrRemovesStaleFiles(t *testing.T) {
	dir := privateDir(t)
	path := filepath.Join(dir, TempPrefix+"orphan")
	data, err := encodeSnapshot(NewSnapshot())
	must(t, err)
	writeFile(t, path, data)
	s, err := Open(dir)
	if s != nil || err == nil || !bytes.Equal(data, readFile(t, path)) {
		t.Fatal("orphaned temporary state was adopted or removed")
	}
	if _, err := os.Lstat(filepath.Join(dir, SnapshotName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing canonical snapshot was silently initialized")
	}

	writeFile(t, filepath.Join(dir, SnapshotName), data)
	foreign := filepath.Join(dir, "unrecognized")
	writeFile(t, foreign, []byte("foreign"))
	s = testStore(t, dir)
	next := NewSnapshot()
	next.Parallel = 2
	must(t, s.Save(next))
	if !bytes.Equal(data, readFile(t, path)) || string(readFile(t, foreign)) != "foreign" {
		t.Fatal("unowned artifacts were removed or modified")
	}
}

func TestStoreRefusesInvalidUpdatesBeforeIO(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	before := testSnapshot(t, dir, 1)
	before.NextOrder = 5
	must(t, s.Save(before))
	data := readFile(t, filepath.Join(dir, SnapshotName))
	s.ops.write = func(*os.File, []byte) (int, error) {
		t.Fatal("invalid mutation reached file IO")
		return 0, nil
	}

	for _, edit := range []func(*Snapshot){
		func(s *Snapshot) { s.Parallel = 0 },
		func(s *Snapshot) { s.NextOrder = 4 },
		func(s *Snapshot) { s.Jobs[0].Order = 2 },
		func(s *Snapshot) { s.Jobs = append(s.Jobs, testJob(t, dir, 2)) },
		func(s *Snapshot) {
			s.Jobs[0].Request.LimitBPS++
			s.Jobs[0].InputHash, _ = s.Jobs[0].Request.hash()
		},
		func(s *Snapshot) {
			*s.Jobs[0].Request.URL += "&new=1"
			s.Jobs[0].InputHash, _ = s.Jobs[0].Request.hash()
		},
	} {
		next := before.clone()
		edit(&next)
		assertPrivateError(t, s.Save(next))
		assertStore(t, s, before)
		if !bytes.Equal(data, readFile(t, filepath.Join(dir, SnapshotName))) {
			t.Fatal("invalid update modified disk state")
		}
	}

	s.ops = defaultStoreOps()
	next := before.clone()
	next.Jobs[0].Order = next.NextOrder
	next.NextOrder++
	must(t, s.Save(next))
	next.Jobs[0].State = Canceled
	next.Jobs[0].Request.URL = nil
	must(t, s.Save(next))
	for _, edit := range []func(*Snapshot){
		func(s *Snapshot) { s.Jobs[0].InputHash = strings.Repeat("f", 64) },
		func(s *Snapshot) { s.Jobs[0].Request.Retries++ },
		func(s *Snapshot) {
			s.Jobs[0].State = Queued
			s.Jobs[0].Request.URL = copyPtr(before.Jobs[0].Request.URL)
		},
	} {
		invalid := next.clone()
		edit(&invalid)
		assertPrivateError(t, s.Save(invalid))
		assertStore(t, s, next)
	}
	next.Jobs = []Job{}
	must(t, s.Save(next))
	next.Jobs = append(next.Jobs, testJob(t, dir, next.NextOrder))
	next.NextOrder++
	must(t, s.Save(next))
}

func TestStoreDetectsChangedArtifacts(t *testing.T) {
	for _, name := range []string{SnapshotName, LockName} {
		for _, kind := range []string{"replacement", "removal", "mode", "hardlink"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				dir := privateDir(t)
				s := testStore(t, dir)
				path := filepath.Join(dir, name)
				switch kind {
				case "replacement":
					replacement := filepath.Join(dir, "replacement")
					writeFile(t, replacement, readFile(t, path))
					must(t, os.Rename(replacement, path))
				case "removal":
					must(t, os.Remove(path))
				case "mode":
					must(t, os.Chmod(path, 0o644))
				case "hardlink":
					must(t, os.Link(path, filepath.Join(dir, "alias")))
				}
				next := NewSnapshot()
				next.Parallel = 2
				if err := s.Save(next); !errors.Is(err, ErrChanged) {
					t.Fatalf("changed artifact did not fault the store: %v", err)
				}
				if _, err := s.Snapshot(); !errors.Is(err, ErrChanged) {
					t.Fatal("faulted store returned an authoritative snapshot")
				}
				if err := s.Save(next); !errors.Is(err, ErrChanged) {
					t.Fatal("faulted store accepted another write")
				}
				if kind == "removal" {
					if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("removed artifact was recreated")
					}
				}
			})
		}
	}

	t.Run("in-place edit", func(t *testing.T) {
		dir := privateDir(t)
		s := testStore(t, dir)
		path := filepath.Join(dir, SnapshotName)
		changed := bytes.ReplaceAll(readFile(t, path), []byte(`"parallel":1`), []byte(`"parallel":2`))
		writeFile(t, path, changed)
		if err := s.Save(NewSnapshot()); !errors.Is(err, ErrChanged) || !bytes.Equal(changed, readFile(t, path)) {
			t.Fatal("in-place canonical edit was overwritten")
		}
	})
}

func TestStoreSerializesAccess(t *testing.T) {
	s := testStore(t, privateDir(t))
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			next := NewSnapshot()
			next.Parallel = i%MaxParallel + 1
			if err := s.Save(next); err != nil {
				t.Error(err)
			}
			if _, err := s.Snapshot(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

type fileInfo struct {
	os.FileInfo
	stat *syscall.Stat_t
}

func (i fileInfo) Sys() any {
	return i.stat
}

func testStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	must(t, err)
	t.Cleanup(func() { must(t, s.Close()) })
	return s
}

func privateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must(t, os.Chmod(dir, 0o700))
	return dir
}

func assertStore(t *testing.T, s *Store, want Snapshot) {
	t.Helper()
	got, err := s.Snapshot()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("committed snapshot differs: %v", err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	must(t, os.WriteFile(path, data, 0o600))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
