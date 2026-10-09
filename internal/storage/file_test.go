package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublish(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "replace"}[overwrite], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			if overwrite {
				write(t, path, "old")
			}

			f, err := Create(path, overwrite)
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() { f.Close() })
			if _, err := f.Write([]byte("complete")); err != nil {
				t.Fatal(err)
			}

			if overwrite {
				assertData(t, path, "old")
			} else {
				assertAbsent(t, path)
			}

			if err := f.Finalize(); err != nil {
				t.Fatal(err)
			}

			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			assertData(t, path, "complete")
			assertAbsent(t, path+PartSuffix)
			if !f.Published() || f.Partial() != "" {
				t.Fatal("incorrect publication result")
			}
		})
	}
}

func TestDestinationAppearsBeforePublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := Create(path, false)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { f.Close() })
	if _, err := f.Write([]byte("download")); err != nil {
		t.Fatal(err)
	}

	write(t, path, "other writer")
	if err := f.Finalize(); err == nil {
		t.Fatal("publication replaced an unexpected destination")
	}

	assertData(t, path, "other writer")
	assertData(t, path+PartSuffix, "download")
	if f.Published() {
		t.Fatal("failed publication reported success")
	}
}

func TestExistingArtifactsArePreserved(t *testing.T) {
	for _, suffix := range []string{"", PartSuffix, ManifestSuffix} {
		t.Run(suffix, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			write(t, path+suffix, "preserve")
			if f, err := Create(path, false); err == nil {
				f.Close()
				t.Fatal("accepted an existing artifact")
			}

			assertData(t, path+suffix, "preserve")
		})
	}
}

func TestOverwriteDoesNotDiscardPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	write(t, path, "old")
	write(t, path+PartSuffix, "partial")
	if f, err := Create(path, true); err == nil {
		f.Close()
		t.Fatal("overwrite discarded partial state")
	}

	assertData(t, path, "old")
	assertData(t, path+PartSuffix, "partial")
}

func TestSingleWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := Create(path, false)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { f.Close() })
	if other, err := Create(path, true); err == nil {
		other.Close()
		t.Fatal("second writer acquired partial file")
	}
}

func TestConcurrentOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	const count = 12
	start := make(chan struct{})
	results := make(chan *File, count)
	for range count {
		go func() {
			<-start
			f, _ := Create(path, false)
			results <- f
		}()
	}

	close(start)
	var owners []*File
	for range count {
		if f := <-results; f != nil {
			owners = append(owners, f)
		}
	}

	for _, f := range owners {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if len(owners) != 1 {
		t.Fatalf("got %d owners, want one", len(owners))
	}
}

func TestAbort(t *testing.T) {
	for _, data := range []string{"", "partial"} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			f, err := Create(path, false)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := f.Write([]byte(data)); err != nil {
				t.Fatal(err)
			}

			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			assertAbsent(t, path)
			if data == "" {
				assertAbsent(t, path+PartSuffix)
			} else {
				assertData(t, path+PartSuffix, data)
			}
		})
	}
}

func TestUnsafePaths(t *testing.T) {
	for _, path := range []string{"", "file" + PartSuffix, "file" + ManifestSuffix} {
		if f, err := Create(path, false); err == nil {
			f.Close()
			t.Fatalf("accepted reserved output %q", path)
		}
	}

	for _, suffix := range []string{"", PartSuffix, ManifestSuffix} {
		t.Run("symlink"+suffix, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file")
			target := filepath.Join(dir, "target")
			write(t, target, "untouched")
			if err := os.Symlink(target, path+suffix); err != nil {
				t.Fatal(err)
			}

			if f, err := Create(path, true); err == nil {
				f.Close()
				t.Fatal("accepted a symlink artifact")
			}

			assertData(t, target, "untouched")
		})
	}

	dir := t.TempDir()
	if f, err := Create(dir, true); err == nil {
		f.Close()
		t.Fatal("accepted directory destination")
	}

	if f, err := Create(filepath.Join(dir, "missing", "file"), false); err == nil {
		f.Close()
		t.Fatal("created missing parent unexpectedly")
	}
}

func TestFailedSyncCannotPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := Create(path, false)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { f.Close() })
	if _, err := f.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}

	if err := f.file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := f.Finalize(); err == nil {
		t.Fatal("reported completion after failed sync")
	}

	assertAbsent(t, path)
	assertData(t, path+PartSuffix, "data")
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertData(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s: data = %q, error = %v; want %q", path, got, err, want)
	}
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s should be absent: %v", path, err)
	}
}
