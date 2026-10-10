package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckpointAndResume(t *testing.T) {
	path, f := startTransfer(t, 10)
	writeBytes(t, f, "abc")

	if err := f.Checkpoint(true); err != nil {
		t.Fatal(err)
	}

	writeBytes(t, f, "TAIL")
	crash(t, f)

	m := inspect(t, path)
	if m.Completed != 3 || m.Checksum != hashText("abc") || m.State != StatePartial {
		t.Fatalf("invalid saved prefix: %+v", m)
	}

	f = reopen(t, path, false)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	assertData(t, path+PartSuffix, "abcTAIL")
	f = reopen(t, path, false)

	if err := f.Continue(); err != nil {
		t.Fatal(err)
	}

	assertData(t, path+PartSuffix, "abc")
	writeBytes(t, f, "defghij")

	if err := f.Finalize(); err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	assertData(t, path, "abcdefghij")
	assertAbsent(t, path+PartSuffix)
	assertAbsent(t, path+ManifestSuffix)
}

func TestPeriodicAndShutdownCheckpoints(t *testing.T) {
	path, f := startTransfer(t, CheckpointBytes+10)
	data := bytes.Repeat([]byte("x"), CheckpointBytes)

	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}

	if m := inspect(t, path); m.Completed != CheckpointBytes {
		t.Fatalf("periodic checkpoint = %d", m.Completed)
	}

	writeBytes(t, f, "end")

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if m := inspect(t, path); m.Completed != CheckpointBytes+3 {
		t.Fatalf("shutdown checkpoint = %d", m.Completed)
	}
}

func TestUnverifiedFinalChunkIsNotCommitted(t *testing.T) {
	path, f := startTransfer(t, 6)
	writeBytes(t, f, "abc")

	if err := f.Checkpoint(true); err != nil {
		t.Fatal(err)
	}

	writeBytes(t, f, "def")

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if m := inspect(t, path); m.Completed != 3 || m.State != StatePartial {
		t.Fatalf("claimed unverified completion: %+v", m)
	}

	assertAbsent(t, path)
	assertData(t, path+PartSuffix, "abcdef")
}

func TestCheckpointSyncFailureKeepsPreviousMetadata(t *testing.T) {
	path, f := startTransfer(t, 10)
	writeBytes(t, f, "abc")

	if err := f.Checkpoint(true); err != nil {
		t.Fatal(err)
	}

	writeBytes(t, f, "de")

	if err := f.file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := f.Checkpoint(true); err == nil {
		t.Fatal("checkpoint succeeded without durable data")
	}

	if m := inspect(t, path); m.Completed != 3 || m.Checksum != hashText("abc") {
		t.Fatalf("advanced failed checkpoint: %+v", m)
	}
}

func TestPublicationRecovery(t *testing.T) {
	for _, phase := range []string{"ready", "linked", "renamed"} {
		t.Run(phase, func(t *testing.T) {
			path, f := startTransfer(t, 6)
			writeBytes(t, f, "abcdef")

			if err := f.prepare(); err != nil {
				t.Fatal(err)
			}

			var err error

			switch phase {
			case "linked":
				err = f.root.Link(f.part, f.name)
			case "renamed":
				err = f.root.Rename(f.part, f.name)
			}

			if err != nil {
				t.Fatal(err)
			}

			crash(t, f)
			f = reopen(t, path, false)

			if err := f.Finalize(); err != nil {
				t.Fatal(err)
			}

			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			assertData(t, path, "abcdef")
			assertAbsent(t, path+PartSuffix)
			assertAbsent(t, path+ManifestSuffix)
		})
	}
}

func TestPublicationRecoveryPreservesForeignDestination(t *testing.T) {
	path, f := startTransfer(t, 6)
	writeBytes(t, f, "abcdef")

	if err := f.prepare(); err != nil {
		t.Fatal(err)
	}

	crash(t, f)
	write(t, path, "foreign")

	if f, err := Open(context.Background(), path, false, true); err == nil {
		f.Close()
		t.Fatal("recovery ignored the destination collision")
	}

	assertData(t, path, "foreign")
	f = reopen(t, path, true)

	if err := f.Finalize(); err != nil {
		t.Fatal(err)
	}

	assertData(t, path, "abcdef")
}

func TestCorruptPrefixAndMissingPartialAreRejected(t *testing.T) {
	for _, change := range []string{"bytes", "short", "oversized", "missing", "symlink"} {
		t.Run(change, func(t *testing.T) {
			path, f := startTransfer(t, 10)
			writeBytes(t, f, "abc")

			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			switch change {
			case "bytes":
				write(t, path+PartSuffix, "bad")
			case "short":
				write(t, path+PartSuffix, "a")
			case "oversized":
				write(t, path+PartSuffix, "abcdefghijkl")
			case "missing", "symlink":
				if err := os.Remove(path + PartSuffix); err != nil {
					t.Fatal(err)
				}

				if change == "symlink" {
					write(t, path, "abc")

					if err := os.Symlink(path, path+PartSuffix); err != nil {
						t.Fatal(err)
					}
				}
			}

			before, err := os.ReadFile(path + ManifestSuffix)
			if err != nil {
				t.Fatal(err)
			}

			if f, err := Open(context.Background(), path, true, true); err == nil {
				f.Close()
				t.Fatal("accepted corrupt local state")
			}

			after, err := os.ReadFile(path + ManifestSuffix)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("changed incompatible metadata")
			}
		})
	}
}

func TestManifestValidation(t *testing.T) {
	cases := map[string]func(*Manifest){
		"version":     func(m *Manifest) { m.Version++ },
		"destination": func(m *Manifest) { m.Destination += ".other" },
		"source":      func(m *Manifest) { m.SourceID = "https://example.org?token=secret" },
		"negative":    func(m *Manifest) { m.Completed = -1 },
		"bounds":      func(m *Manifest) { m.Completed = 11 },
		"full prefix": func(m *Manifest) { m.Completed = 10 },
		"ready gap":   func(m *Manifest) { m.State = StateReady },
		"checksum":    func(m *Manifest) { m.Checksum = "invalid" },
		"weak etag":   func(m *Manifest) { m.ETag = `W/"version"` },
		"encoding":    func(m *Manifest) { m.Encoding = "invalid" },
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			path, f := startTransfer(t, 10)
			writeBytes(t, f, "abc")

			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			m := inspect(t, path)
			change(&m)
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}

			write(t, path+ManifestSuffix, string(data))

			if _, err := Inspect(path); err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("missing or unsafe manifest error: %v", err)
			}
		})
	}
}

func TestManifestJSONValidation(t *testing.T) {
	for _, kind := range []string{"duplicate", "missing", "unknown", "null", "trailing", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			path, f := startTransfer(t, 10)
			writeBytes(t, f, "abc")

			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			data, err := os.ReadFile(path + ManifestSuffix)
			if err != nil {
				t.Fatal(err)
			}

			text := string(data)

			switch kind {
			case "duplicate":
				text = strings.Replace(text, "{", `{"schema_version": 1,`, 1)
			case "missing":
				text = strings.Replace(text, `"length": 10,`, "", 1)
			case "unknown":
				text = strings.Replace(text, `"length"`, `"unknown"`, 1)
			case "null":
				var fields map[string]any
				if err := json.Unmarshal(data, &fields); err != nil {
					t.Fatal(err)
				}

				fields["etag"] = nil
				data, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}

				text = string(data)
			case "trailing":
				text += "{}"
			case "oversized":
				text += strings.Repeat(" ", maxManifestSize)
			}

			write(t, path+ManifestSuffix, text)

			if _, err := Inspect(path); err == nil {
				t.Fatal("accepted invalid JSON/schema")
			}
		})
	}
}

func TestLockSurvivesCleanupAndReleasesOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := Create(path, false)
	if err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path + LockSuffix)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	f, err = Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	after, err := os.Stat(path + LockSuffix)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("lock inode was replaced between owners")
	}
}

func TestExplicitResumeNeedsState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")

	if f, err := Open(context.Background(), path, false, true); err == nil {
		f.Close()
		t.Fatal("explicit resume created a fresh transfer")
	}

	assertAbsent(t, path+PartSuffix)
	assertAbsent(t, path+ManifestSuffix)
}

func startTransfer(t *testing.T, length int64) (string, *File) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	f, err := Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	err = f.Begin(Manifest{
		SourceID: hashText("source"), EffectiveID: hashText("effective"),
		Length: length, ETag: `"version"`, Encoding: "identity",
	})

	if err != nil {
		t.Fatal(err)
	}

	return path, f
}

func reopen(t *testing.T, path string, overwrite bool) *File {
	t.Helper()
	f, err := Open(context.Background(), path, overwrite, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	return f
}

func writeBytes(t *testing.T, f *File, data string) {
	t.Helper()

	if _, err := f.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
}

func crash(t *testing.T, f *File) {
	t.Helper()
	f.active = false

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func inspect(t *testing.T, path string) Manifest {
	t.Helper()
	m, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

func hashText(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}
