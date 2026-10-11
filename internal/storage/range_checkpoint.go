package storage

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"

	"yoinkdl/internal/ranges"
)

func (f *File) BeginRanges(m Manifest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.manifest != nil || f.written != 0 || f.file == nil || (m.Version != 0 && m.Version != RangeManifestVersion) {
		return errors.New("cannot initialize range state for an existing transfer")
	}

	if m.ChunkSize == 0 {
		size, err := ranges.ChunkSize(m.Length)
		if err != nil {
			return err
		}

		m.ChunkSize = size
	}

	m.Version = RangeManifestVersion
	m.Destination = f.path
	m.Completed = 0
	m.Checksum = ""
	m.Ranges = []Chunk{}
	m.State = StatePartial

	if err := m.validate(f.path); err != nil {
		return err
	}

	if err := f.file.Sync(); err != nil {
		return err
	}

	if err := f.saveManifest(m); err != nil {
		return err
	}

	f.active = true
	return nil
}

func (f *File) CheckpointRange(ctx context.Context, chunk Chunk) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writers[chunk.Start] != nil {
		return errors.New("chunk still belongs to an active range writer")
	}

	return f.checkpointRange(ctx, chunk)
}

func (f *File) checkpointRange(ctx context.Context, chunk Chunk) error {
	if !f.active || f.file == nil || f.manifest == nil ||
		f.manifest.Version != RangeManifestVersion || f.manifest.State != StatePartial {
		return errors.New("range transfer is not active")
	}

	m := f.manifest.clone()
	m.Ranges = append(m.Ranges, chunk)
	slices.SortFunc(m.Ranges, func(a, b Chunk) int { return cmp.Compare(a.Start, b.Start) })
	covered, err := ranges.Covered(m.Length, m.ChunkSize, m.Spans())
	if err != nil {
		return err
	}

	m.Completed = covered
	if err := m.validate(f.path); err != nil {
		return err
	}

	if err := f.checkIdentity(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("sync completed chunk: %w", err)
	}

	if err := f.verifyChunk(ctx, chunk); err != nil {
		return err
	}

	if err := f.saveManifest(m); err != nil {
		return err
	}

	f.written = covered
	return nil
}

func (f *File) ContinueRanges() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.manifest == nil || f.manifest.Version != RangeManifestVersion || f.manifest.State != StatePartial || f.file == nil {
		return errors.New("saved transfer cannot resume as ranges")
	}

	if err := f.checkIdentity(); err != nil {
		return err
	}

	f.active = true
	return nil
}

func (f *File) prepareRanges(ctx context.Context) error {
	m := f.manifest.clone()
	if !f.active || m.Completed != m.Length {
		return errors.New("cannot publish incomplete range coverage")
	}

	if err := m.validate(f.path); err != nil {
		return err
	}

	info, err := f.file.Stat()
	if err != nil {
		return err
	}

	if info.Size() != m.Length {
		return errors.New("range file size does not match the representation")
	}

	if err := f.verifyRanges(ctx, m); err != nil {
		return err
	}

	m.Checksum, err = f.hashSpan(ctx, ranges.Span{End: m.Length})
	if err != nil {
		return err
	}

	m.State = StateReady
	return f.saveManifest(m)
}

func (f *File) verifyRanges(ctx context.Context, m Manifest) error {
	for _, chunk := range m.Ranges {
		if err := f.verifyChunk(ctx, chunk); err != nil {
			return err
		}
	}

	if m.State == StateReady {
		sum, err := f.hashSpan(ctx, ranges.Span{End: m.Length})
		if err != nil {
			return err
		}

		if sum != m.Checksum {
			return errors.New("saved file does not match the ready checksum")
		}
	}

	return ctx.Err()
}

func (f *File) verifyChunk(ctx context.Context, chunk Chunk) error {
	sum, err := f.hashSpan(ctx, chunk.Span)
	if err != nil {
		return err
	}

	if sum != chunk.Checksum {
		return errors.New("saved chunk does not match its checkpoint checksum")
	}

	return nil
}

func (f *File) hashSpan(ctx context.Context, span ranges.Span) (string, error) {
	digest := sha256.New()
	reader := &checkedReader{ctx: ctx, reader: io.NewSectionReader(f.file, span.Start, span.End-span.Start)}

	if _, err := io.CopyN(digest, reader, span.End-span.Start); err != nil {
		return "", fmt.Errorf("read checkpoint interval: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}
