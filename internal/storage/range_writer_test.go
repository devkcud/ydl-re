package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"yoinkdl/internal/ranges"
)

func TestRangeWriterOwnership(t *testing.T) {
	path, f := startRanges(t, 6, 3)
	span := ranges.Span{End: 3}
	w, err := f.ClaimRange(span)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for _, other := range []ranges.Span{span, {Start: 1, End: 4}, {End: 6}} {
		if writer, err := f.ClaimRange(other); err == nil {
			writer.Close()
			t.Fatalf("accepted duplicate or overlapping ownership: %v", other)
		}
	}

	if err := f.Close(); err == nil {
		t.Fatal("closed a file with an active writer")
	}

	if err := f.Finalize(); err == nil {
		t.Fatal("published a file with an active writer")
	}

	if other, err := Open(context.Background(), path, false, true); err == nil {
		other.Close()
		t.Fatal("released process ownership while a writer was active")
	}

	if _, err := w.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}

	if err := w.Commit(context.Background()); err == nil {
		t.Fatal("checkpointed an incomplete chunk")
	}

	if err := f.CheckpointRange(context.Background(), Chunk{Span: span, Checksum: hashText("abc")}); err == nil {
		t.Fatal("bypassed active writer ownership")
	}

	w.Close()
	if _, err := w.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("stale writer error = %v", err)
	}

	next, err := f.ClaimRange(span)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if _, err := next.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}

	if err := next.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	if writer, err := f.ClaimRange(span); err == nil {
		writer.Close()
		t.Fatal("allowed overwriting a checkpointed chunk")
	}

	assertData(t, path+PartSuffix, "abc")
	if m := inspect(t, path); m.Completed != 3 {
		t.Fatalf("saved bytes = %d", m.Completed)
	}
}

func TestRangeWriterFailurePreservesCheckpoint(t *testing.T) {
	for _, mode := range []string{"overflow", "cancel", "disk"} {
		t.Run(mode, func(t *testing.T) {
			path, f := startRanges(t, 6, 3)
			w, err := f.ClaimRange(ranges.Span{Start: 3, End: 6})
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			before := readBytes(t, path+ManifestSuffix)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if mode == "overflow" {
				if n, err := w.Write([]byte("toolong")); err == nil || n != 0 {
					t.Fatal("wrote beyond the assigned offset")
				}
			} else {
				if _, err := w.Write([]byte("def")); err != nil {
					t.Fatal(err)
				}

				if mode == "cancel" {
					cancel()
				} else if err := f.file.Close(); err != nil {
					t.Fatal(err)
				}
			}

			if err := w.Commit(ctx); err == nil {
				t.Fatal("checkpointed a failed writer")
			}

			if !bytes.Equal(before, readBytes(t, path+ManifestSuffix)) {
				t.Fatal("failed writer advanced saved coverage")
			}

			assertAbsent(t, path)
		})
	}
}

func TestConcurrentRangeWriters(t *testing.T) {
	const size = 32 * 1024
	const count = 16
	path, f := startRanges(t, count*size, size)
	var group sync.WaitGroup
	errs := make(chan error, count)
	want := make([]byte, count*size)

	for i := range count {
		data := bytes.Repeat([]byte{byte(i)}, size)
		copy(want[i*size:], data)
		group.Go(func() {
			w, err := f.ClaimRange(ranges.Span{Start: int64(i * size), End: int64((i + 1) * size)})
			if err != nil {
				errs <- err
				return
			}
			defer w.Close()

			for start := 0; start < size; start += 1024 {
				if _, err := w.Write(data[start : start+1024]); err != nil {
					errs <- err
					return
				}
			}

			errs <- w.Commit(context.Background())
		})
	}

	for range count {
		if err := <-errs; err != nil {
			t.Error(err)
		}

		m, _ := f.Metadata()
		if m.Completed < 0 || m.Completed > count*size {
			t.Errorf("invalid concurrent coverage: %d", m.Completed)
		}
	}
	group.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.FinalizeContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled publication error = %v", err)
	}
	assertAbsent(t, path)

	if err := f.FinalizeContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := readBytes(t, path); !bytes.Equal(got, want) {
		t.Fatal("concurrent writes produced gaps, overlap, or wrong offsets")
	}
}

func TestConcurrentClaimsHaveOneOwner(t *testing.T) {
	_, f := startRanges(t, 3, 3)
	const count = 12
	results := make(chan *RangeWriter, count)
	start := make(chan struct{})
	for range count {
		go func() {
			<-start
			w, _ := f.ClaimRange(ranges.Span{End: 3})
			results <- w
		}()
	}
	close(start)

	var owners []*RangeWriter
	for range count {
		if w := <-results; w != nil {
			owners = append(owners, w)
		}
	}

	for _, w := range owners {
		w.Close()
	}

	if len(owners) != 1 {
		t.Fatalf("chunk acquired %d owners", len(owners))
	}
}
