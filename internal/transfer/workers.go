package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

type workerEvent struct {
	bytes   int64
	workers int
}

func receiveRanges(ctx context.Context, client *http.Client, source *url.URL, id representation, f *storage.File,
	count int, policy retryPolicy, progress func(Snapshot)) (int64, error) {
	m, saved := f.Metadata()
	if !saved || m.Version != storage.RangeManifestVersion || count < 1 || count > MaxConnections ||
		policy.limit < 0 || policy.limit > MaxRetries {
		return 0, errors.New("invalid range worker configuration")
	}

	if m.SourceID != id.sourceID || m.EffectiveID != id.effectiveID || m.Length != id.length ||
		m.ETag != id.etag || m.Encoding != "identity" {
		return m.Completed, errors.New("worker identity does not match the saved representation")
	}

	plan, err := ranges.Missing(m.Length, m.ChunkSize, m.Spans())
	if err != nil {
		return m.Completed, err
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	snapshot := Snapshot{Bytes: m.Completed, Total: m.Length, Resumed: m.Completed}
	if progress != nil {
		progress(snapshot)
	}

	if cause := context.Cause(ctx); cause != nil {
		return snapshot.Bytes, cause
	}

	count = min(count, len(plan))
	jobs := make(chan ranges.Span, len(plan))
	events := make(chan workerEvent, count)
	for _, span := range plan {
		jobs <- span
	}
	close(jobs)

	var group sync.WaitGroup
	for range count {
		group.Go(func() {
			buf := make([]byte, bufferSize)
			for span := range jobs {
				if ctx.Err() != nil {
					return
				}

				events <- workerEvent{workers: 1}
				var received int64
				err := policy.run(ctx, func(attempt int) error {
					if attempt > 0 && received > 0 {
						events <- workerEvent{bytes: -received}
						received = 0
					}
					return receiveRange(ctx, client, source, id, f, span, buf, func(n int) {
						received += int64(n)
						events <- workerEvent{bytes: int64(n)}
					}, func(err error) {
						if !policy.canRetry(ctx, attempt, err) {
							cancel(err)
						}
					})
				})
				if err != nil {
					cancel(err)
				}

				events <- workerEvent{workers: -1}
				if err != nil {
					return
				}
			}
		})
	}

	go func() {
		group.Wait()
		close(events)
	}()

	for event := range events {
		snapshot.Bytes += event.bytes
		snapshot.Workers += event.workers
		if progress != nil {
			progress(snapshot)
		}
	}

	return snapshot.Bytes, context.Cause(ctx)
}

func receiveRange(ctx context.Context, client *http.Client, source *url.URL, id representation, f *storage.File,
	span ranges.Span, buf []byte, progress func(int), fail context.CancelCauseFunc) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	req, err := id.request(ctx, source, span)
	if err != nil {
		return err
	}

	resp, err := requestOnce(ctx, client, req)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}

		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		resp.Body.Close()
		return reprobe(ctx, client, req)
	}

	if err := id.validate(resp, span); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	w, err := f.ClaimRange(span)
	if err != nil {
		return err
	}
	defer w.Close()
	defer func() {
		if err != nil {
			fail(err)
		}
	}()
	var received int64

	for {
		n, readErr := resp.Body.Read(buf)
		if cause := context.Cause(ctx); cause != nil {
			return bodyFailure(cause)
		}

		if n > 0 {
			if int64(n) > span.End-span.Start-received {
				return errors.New("range response exceeded its assigned interval")
			}

			written, err := w.Write(buf[:n])
			received += int64(written)
			progress(written)
			if err != nil {
				return fmt.Errorf("write range bytes: %w", err)
			}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}

		if readErr != nil {
			return bodyFailure(readErr)
		}
	}

	if received != span.End-span.Start {
		return bodyFailure(io.ErrUnexpectedEOF)
	}

	return w.Commit(ctx)
}
