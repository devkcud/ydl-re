package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"
	"sync"

	"yoinkdl/internal/ranges"
)

type RangeWriter struct {
	mu     sync.Mutex
	owner  *File
	file   *os.File
	span   ranges.Span
	offset int64
	digest hash.Hash
	err    error
	closed bool
}

func (f *File) ClaimRange(span ranges.Span) (*RangeWriter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.active || f.file == nil || f.manifest == nil ||
		f.manifest.Version != RangeManifestVersion || f.manifest.State != StatePartial {
		return nil, errors.New("range transfer is not active")
	}

	if _, err := ranges.Covered(f.manifest.Length, f.manifest.ChunkSize, []ranges.Span{span}); err != nil {
		return nil, err
	}

	if f.writers[span.Start] != nil {
		return nil, errors.New("chunk already has a writer")
	}

	for _, chunk := range f.manifest.Ranges {
		if chunk.Start == span.Start {
			return nil, errors.New("completed chunk cannot be overwritten")
		}
	}

	if err := f.checkIdentity(); err != nil {
		return nil, err
	}

	w := &RangeWriter{owner: f, file: f.file, span: span, offset: span.Start, digest: sha256.New()}
	if f.writers == nil {
		f.writers = make(map[int64]*RangeWriter)
	}

	f.writers[span.Start] = w
	return w, nil
}

func (w *RangeWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}

	if w.err != nil {
		return 0, w.err
	}

	if int64(len(data)) > w.span.End-w.offset {
		w.err = errors.New("write exceeds the assigned chunk")
		return 0, w.err
	}

	n, err := w.file.WriteAt(data, w.offset)
	w.offset += int64(n)
	w.digest.Write(data[:n])
	if n != len(data) && err == nil {
		err = io.ErrShortWrite
	}

	w.err = err
	return n, err
}

func (w *RangeWriter) Commit(ctx context.Context) error {
	w.owner.mu.Lock()
	defer w.owner.mu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return os.ErrClosed
	}

	if w.err != nil {
		return w.err
	}

	if w.offset != w.span.End {
		return errors.New("cannot checkpoint an incomplete chunk")
	}

	chunk := Chunk{Span: w.span, Checksum: hex.EncodeToString(w.digest.Sum(nil))}
	if err := w.owner.checkpointRange(ctx, chunk); err != nil {
		return err
	}

	w.closed = true
	delete(w.owner.writers, w.span.Start)
	return nil
}

func (w *RangeWriter) Close() error {
	w.owner.mu.Lock()
	defer w.owner.mu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.closed = true
		delete(w.owner.writers, w.span.Start)
	}

	return nil
}
