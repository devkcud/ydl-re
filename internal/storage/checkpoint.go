package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const CheckpointBytes = 4 * 1024 * 1024
const checkpointInterval = time.Second

func (f *File) Begin(m Manifest) error {
	if f.manifest != nil || f.written != 0 || f.file == nil {
		return errors.New("cannot initialize an existing transfer")
	}

	f.digest = sha256.New()
	m.Version = ManifestVersion
	m.Destination = f.path
	m.Completed = 0
	m.Checksum = hex.EncodeToString(f.digest.Sum(nil))
	m.State = StatePartial

	if err := f.file.Sync(); err != nil {
		return err
	}

	if err := f.saveManifest(m); err != nil {
		return err
	}

	f.active = true
	f.last = time.Now()
	return nil
}

func (f *File) Continue() error {
	if f.manifest == nil || f.manifest.State != StatePartial || !f.manifest.Resumable() || f.file == nil {
		return errors.New("saved transfer cannot resume")
	}

	if err := f.checkIdentity(); err != nil {
		return err
	}

	if err := f.file.Truncate(f.manifest.Completed); err != nil {
		return err
	}

	if _, err := f.file.Seek(f.manifest.Completed, io.SeekStart); err != nil {
		return err
	}

	f.active = true
	f.last = time.Now()
	return nil
}

func (f *File) Checkpoint(force bool) error {
	if !f.active || f.manifest == nil || f.manifest.State == StateReady || f.written == f.manifest.Completed {
		return nil
	}

	if f.manifest.Length > 0 && f.written == f.manifest.Length {
		return nil
	}

	if !force && f.written-f.manifest.Completed < CheckpointBytes && time.Since(f.last) < checkpointInterval {
		return nil
	}

	if err := f.checkIdentity(); err != nil {
		return err
	}

	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("sync checkpoint bytes: %w", err)
	}

	m := *f.manifest
	m.Completed = f.written
	m.Checksum = hex.EncodeToString(f.digest.Sum(nil))

	if err := f.saveManifest(m); err != nil {
		return err
	}

	f.last = time.Now()
	return nil
}

func (f *File) prepare() error {
	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("sync partial file: %w", err)
	}

	if f.manifest == nil || f.manifest.State == StateReady {
		return nil
	}

	if !f.active || (f.manifest.Length >= 0 && f.written != f.manifest.Length) {
		return errors.New("cannot publish an incomplete transfer")
	}

	m := *f.manifest
	m.State = StateReady
	m.Length = f.written
	m.Completed = f.written
	m.Checksum = hex.EncodeToString(f.digest.Sum(nil))
	return f.saveManifest(m)
}

func (f *File) restore(ctx context.Context) error {
	m, err := readManifest(f.root, f.name+ManifestSuffix, f.path)
	if err != nil {
		return err
	}

	f.manifest = &m
	f.file, err = openRegular(f.root, f.part, os.O_RDWR)

	if errors.Is(err, os.ErrNotExist) && m.State == StateReady {
		f.file, err = openRegular(f.root, f.name, os.O_RDONLY)
		f.fromFinal = err == nil
	} else {
		f.partial = err == nil
	}

	if err != nil {
		return fmt.Errorf("open saved transfer bytes: %w", err)
	}

	f.info, err = f.file.Stat()
	if err != nil {
		return err
	}

	size := f.info.Size()
	if size < m.Completed || (m.Length >= 0 && size > m.Length) || (m.State == StateReady && size != m.Length) {
		return errors.New("saved file size does not match the checkpoint")
	}

	if !f.fromFinal {
		info, err := f.root.Lstat(f.name)
		linked := err == nil && m.State == StateReady && os.SameFile(info, f.info)

		if !linked {
			if err := f.checkDestination(); err != nil {
				return err
			}
		}
	}

	f.digest = sha256.New()
	reader := &checkedReader{ctx: ctx, reader: f.file}

	if _, err := io.CopyN(f.digest, reader, m.Completed); err != nil {
		return fmt.Errorf("read checkpoint prefix: %w", err)
	}

	if hex.EncodeToString(f.digest.Sum(nil)) != m.Checksum {
		return errors.New("saved bytes do not match the checkpoint checksum")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	f.written = m.Completed
	return nil
}

type checkedReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *checkedReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return r.reader.Read(data)
}
