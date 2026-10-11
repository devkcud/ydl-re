package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"yoinkdl/internal/ranges"
)

func TestDisjointCheckpointRecovery(t *testing.T) {
	path, f := startRanges(t, 10, 3)
	putChunk(t, f, 6, "ghi")
	putChunk(t, f, 0, "abc")
	writeAt(t, f, 3, "BAD!")
	writeAt(t, f, 6, "ghi")
	crash(t, f)

	m := inspect(t, path)
	if m.Version != RangeManifestVersion || m.Completed != 6 || m.Checksum != "" {
		t.Fatalf("invalid disjoint checkpoint: %+v", m)
	}

	want := []ranges.Span{{Start: 0, End: 3}, {Start: 6, End: 9}}
	if !reflect.DeepEqual(m.Spans(), want) {
		t.Fatalf("completed = %v; want %v", m.Spans(), want)
	}

	f = reopen(t, path, false)
	before := readBytes(t, path+ManifestSuffix)
	if err := f.Continue(); err == nil {
		t.Fatal("range state used the sequential truncation path")
	}

	if _, err := f.Write([]byte("wrong")); err == nil {
		t.Fatal("range state accepted sequential bytes")
	}

	if err := f.Finalize(); err == nil {
		t.Fatal("published incomplete coverage from a sparse file")
	}

	assertData(t, path+PartSuffix, "abcBADghi")
	if !bytes.Equal(before, readBytes(t, path+ManifestSuffix)) {
		t.Fatal("refused operations changed the checkpoint")
	}

	m, _ = f.Metadata()
	m.Ranges[0].Checksum = hashText("bad")
	saved, _ := f.Metadata()
	if saved.Ranges[0].Checksum != hashText("abc") {
		t.Fatal("metadata exposed mutable checkpoint ownership")
	}

	missing, err := ranges.Missing(saved.Length, saved.ChunkSize, saved.Spans())
	if err != nil || !reflect.DeepEqual(missing, []ranges.Span{{Start: 3, End: 6}, {Start: 9, End: 10}}) {
		t.Fatalf("missing = %v, error = %v", missing, err)
	}

	if err := f.ContinueRanges(); err != nil {
		t.Fatal(err)
	}

	putChunk(t, f, 9, "j")
	putChunk(t, f, 3, "def")
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

func TestRangeShutdownPreservesUnacknowledgedBytes(t *testing.T) {
	for _, data := range []string{"", "bad"} {
		t.Run(data, func(t *testing.T) {
			path, f := startRanges(t, 6, 3)
			writeAt(t, f, 3, data)
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			m := inspect(t, path)
			if m.Completed != 0 || len(m.Ranges) != 0 || m.State != StatePartial {
				t.Fatalf("shutdown acknowledged unverified bytes: %+v", m)
			}

			f = reopen(t, path, false)
			if err := f.Finalize(); err == nil {
				t.Fatal("unverified file length authorized publication")
			}

			assertAbsent(t, path)
			if data != "" {
				assertData(t, path+PartSuffix, "\x00\x00\x00bad")
			}
		})
	}
}

func TestRangeCheckpointRefusalPreservesMetadata(t *testing.T) {
	for _, mode := range []string{"checksum", "short", "duplicate", "partial", "out of bounds", "cancel", "sync", "publish", "identity"} {
		t.Run(mode, func(t *testing.T) {
			path, f := startRanges(t, 9, 3)
			putChunk(t, f, 0, "abc")
			before := readBytes(t, path+ManifestSuffix)
			writeAt(t, f, 3, "def")
			chunk := Chunk{Span: ranges.Span{Start: 3, End: 6}, Checksum: hashText("def")}
			ctx := context.Background()

			switch mode {
			case "checksum":
				chunk.Checksum = hashText("bad")
			case "short":
				if err := f.file.Truncate(5); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				chunk = Chunk{Span: ranges.Span{End: 3}, Checksum: hashText("abc")}
			case "partial":
				chunk.End = 5
			case "out of bounds":
				chunk.End = 10
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "sync":
				if err := f.file.Close(); err != nil {
					t.Fatal(err)
				}
			case "publish":
				if err := os.Rename(path+ManifestSuffix, path+".saved"); err != nil {
					t.Fatal(err)
				}

				if err := os.Mkdir(path+ManifestSuffix, 0o700); err != nil {
					t.Fatal(err)
				}
			case "identity":
				if err := os.Rename(path+PartSuffix, path+".old"); err != nil {
					t.Fatal(err)
				}

				write(t, path+PartSuffix, "foreign")
			}

			if err := f.CheckpointRange(ctx, chunk); err == nil {
				t.Fatal("accepted an invalid range checkpoint")
			}

			m, _ := f.Metadata()
			if m.Completed != 3 || len(m.Ranges) != 1 || m.Ranges[0].Checksum != hashText("abc") {
				t.Fatalf("failed checkpoint changed in-memory authority: %+v", m)
			}

			name := path + ManifestSuffix
			if mode == "publish" {
				name = path + ".saved"
			}

			if !bytes.Equal(before, readBytes(t, name)) {
				t.Fatal("failed checkpoint changed saved metadata")
			}

			assertAbsent(t, path)
		})
	}
}

func TestRangeRestoreRejectsCorruptBytes(t *testing.T) {
	for _, mode := range []string{"chunk", "short", "oversized", "missing", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			path, f := startRanges(t, 10, 3)
			putChunk(t, f, 6, "ghi")
			crash(t, f)
			before := readBytes(t, path+ManifestSuffix)
			ctx := context.Background()

			switch mode {
			case "chunk":
				write(t, path+PartSuffix, "abcdefBAD")
			case "short":
				write(t, path+PartSuffix, "abcdef")
			case "oversized":
				write(t, path+PartSuffix, "abcdefghijk")
			case "missing":
				if err := os.Remove(path + PartSuffix); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			data, _ := os.ReadFile(path + PartSuffix)
			if other, err := Open(ctx, path, false, true); err == nil {
				other.Close()
				t.Fatal("accepted invalid local range state")
			} else if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}

			after, _ := os.ReadFile(path + PartSuffix)
			if !bytes.Equal(before, readBytes(t, path+ManifestSuffix)) || !bytes.Equal(data, after) {
				t.Fatal("failed recovery changed existing artifacts")
			}
		})
	}
}

func TestRangePublicationRecovery(t *testing.T) {
	for _, phase := range []string{"ready", "linked", "renamed", "empty"} {
		t.Run(phase, func(t *testing.T) {
			data := "abcdefg"
			if phase == "empty" {
				data = ""
			}

			path, f := startRanges(t, int64(len(data)), 3)
			if data != "" {
				putChunk(t, f, 6, "g")
				putChunk(t, f, 3, "def")
				putChunk(t, f, 0, "abc")
			}

			if err := f.prepare(); err != nil {
				t.Fatal(err)
			}

			m := inspect(t, path)
			if m.State != StateReady || m.Checksum != hashText(data) {
				t.Fatalf("invalid ready checksum: %+v", m)
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

			assertData(t, path, data)
			assertAbsent(t, path+PartSuffix)
			assertAbsent(t, path+ManifestSuffix)
		})
	}
}

func TestRangeFinalizationRechecksCoverageAndBytes(t *testing.T) {
	for _, mode := range []string{"gap", "corruption", "extra", "foreign"} {
		t.Run(mode, func(t *testing.T) {
			path, f := startRanges(t, 6, 3)
			putChunk(t, f, 3, "def")
			if mode != "gap" {
				putChunk(t, f, 0, "abc")
			}

			switch mode {
			case "corruption":
				writeAt(t, f, 0, "BAD")
			case "extra":
				writeAt(t, f, 6, "x")
			case "foreign":
				write(t, path, "foreign")
			}

			if err := f.Finalize(); err == nil {
				t.Fatal("published an invalid or conflicting range transfer")
			}

			if mode == "foreign" {
				assertData(t, path, "foreign")
			} else {
				assertAbsent(t, path)
			}
		})
	}
}

func TestPrefixStateCannotBecomeRangeState(t *testing.T) {
	path, f := startTransfer(t, 10)
	writeBytes(t, f, "abc")
	if err := f.Checkpoint(true); err != nil {
		t.Fatal(err)
	}

	before := readBytes(t, path+ManifestSuffix)
	if err := f.ContinueRanges(); err == nil {
		t.Fatal("activated prefix state as ranges")
	}

	if err := f.BeginRanges(Manifest{Length: 10}); err == nil {
		t.Fatal("replaced prefix state with ranges")
	}

	if err := f.CheckpointRange(context.Background(), Chunk{Span: ranges.Span{End: 3}, Checksum: hashText("abc")}); err == nil {
		t.Fatal("added ranges to prefix state")
	}

	if !bytes.Equal(before, readBytes(t, path+ManifestSuffix)) {
		t.Fatal("changed the version-1 format")
	}
}

func startRanges(t *testing.T, length, size int64) (string, *File) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	f, err := Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	err = f.BeginRanges(Manifest{
		SourceID: hashText("source"), EffectiveID: hashText("effective"),
		Length: length, ETag: `"version"`, Encoding: "identity", ChunkSize: size,
	})
	if err != nil {
		t.Fatal(err)
	}

	return path, f
}

func putChunk(t *testing.T, f *File, start int64, data string) {
	t.Helper()
	writeAt(t, f, start, data)
	chunk := Chunk{Span: ranges.Span{Start: start, End: start + int64(len(data))}, Checksum: hashText(data)}
	if err := f.CheckpointRange(context.Background(), chunk); err != nil {
		t.Fatal(err)
	}
}

func writeAt(t *testing.T, f *File, start int64, data string) {
	t.Helper()
	if _, err := f.file.WriteAt([]byte(data), start); err != nil {
		t.Fatal(err)
	}
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}
