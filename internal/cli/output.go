package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
)

const outputTimeout = 5 * time.Second

type recordWriter struct {
	out      io.Writer
	release  func() error
	deadline func(time.Time) error
}

func newRecordWriter(out io.Writer) (*recordWriter, error) {
	w := &recordWriter{out: out}
	if file, ok := out.(*os.File); ok {
		var err error
		w.out, w.release, err = openOutput(file)
		if err != nil {
			return nil, err
		}
	}

	if file, ok := w.out.(interface{ SetWriteDeadline(time.Time) error }); ok {
		err := file.SetWriteDeadline(time.Time{})
		if err == nil {
			w.deadline = file.SetWriteDeadline
		} else if !errors.Is(err, os.ErrNoDeadline) {
			return nil, errors.Join(err, w.Close())
		}
	}

	return w, nil
}

func (w *recordWriter) Write(ctx context.Context, data []byte) (n int, err error) {
	if cause := context.Cause(ctx); cause != nil {
		return 0, cause
	}

	if w.deadline != nil {
		if err := w.deadline(time.Now().Add(outputTimeout)); err != nil {
			return 0, err
		}

		done := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			w.deadline(time.Now())
			close(done)
		})
		defer func() {
			if !stop() {
				<-done
			}
			err = errors.Join(err, w.deadline(time.Time{}))
		}()
	}

	n, err = w.out.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil && ctx.Err() != nil {
		err = context.Cause(ctx)
	}

	return n, err
}

func (w *recordWriter) Close() error {
	if w.release == nil {
		return nil
	}

	close := w.release
	w.release = nil
	return close()
}
