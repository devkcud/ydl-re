package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"yoinkdl/internal/storage"
)

const DefaultTimeout = 30 * time.Second
const bufferSize = 64 * 1024

type Request struct {
	URL       string
	Output    string
	Overwrite bool
	Resume    bool
	Timeout   time.Duration
}

type Snapshot struct {
	Bytes   int64
	Total   int64
	Resumed int64
}

type Result struct {
	Path      string
	Partial   string
	Bytes     int64
	Resumed   int64
	Published bool
}

func Download(ctx context.Context, req Request, progress func(Snapshot)) (result Result, err error) {
	u, err := parseURL(req.URL)
	if err != nil {
		return result, err
	}

	if req.Timeout < 0 {
		return result, errors.New("timeout must be positive")
	}

	if req.Timeout == 0 {
		req.Timeout = DefaultTimeout
	}

	if req.Output == "" {
		req.Output = filename(u)
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}

	f, err := storage.Open(ctx, req.Output, req.Overwrite, req.Resume)
	if err != nil {
		return result, err
	}

	result.Path = f.Path()
	defer func() {
		err = errors.Join(err, f.Close())
		result.Partial = f.Partial()
		result.Published = f.Published()
	}()

	m, saved := f.Metadata()
	if saved {
		if m.SourceID != fingerprint(u) {
			return result, errors.New("source URL does not match the saved transfer; existing state is preserved")
		}

		result.Bytes = m.Completed

		if m.State == storage.StateReady {
			result.Resumed = m.Completed
			if progress != nil {
				progress(Snapshot{Bytes: m.Completed, Total: m.Length, Resumed: m.Completed})
			}

			if err := ctx.Err(); err != nil {
				return result, err
			}

			return result, f.Finalize()
		}

		if !m.Resumable() {
			return result, errors.New("saved transfer needs a known length, identity encoding, and strong ETag to resume")
		}
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return result, errors.New("cannot construct HTTP request")
	}

	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "yoinkdl")

	if saved {
		if m.Completed > 0 {
			request.Header.Set("Range", fmt.Sprintf("bytes=%d-", m.Completed))
			request.Header.Set("If-Range", m.ETag)
		} else {
			request.Header.Set("If-Match", m.ETag)
		}
	}

	client := newClient(req.Timeout)
	defer client.CloseIdleConnections()
	resp, err := client.Do(request)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return result, cause
		}

		return result, requestError(err)
	}

	defer resp.Body.Close()

	if saved {
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			resp.Body.Close()
			return result, reprobe(ctx, client, request)
		}

		if err := validateResume(resp, m); err != nil {
			return result, err
		}

		if err := ctx.Err(); err != nil {
			return result, err
		}

		if err := f.Continue(); err != nil {
			return result, err
		}

		result.Resumed = m.Completed
	} else {
		if resp.StatusCode != http.StatusOK {
			return result, fmt.Errorf("HTTP %d %s; expected a complete 200 response", resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		m = responseMetadata(resp, u)
		if err := f.Begin(m); err != nil {
			return result, err
		}
	}

	if progress != nil {
		progress(Snapshot{Bytes: result.Bytes, Total: m.Length, Resumed: result.Resumed})
	}

	timer := time.AfterFunc(req.Timeout, func() {
		cancel(fmt.Errorf("response body idle for %s: %w", req.Timeout, context.DeadlineExceeded))
	})

	timer.Stop()
	defer timer.Stop()
	buf := make([]byte, bufferSize)
	for {
		if cause := context.Cause(ctx); cause != nil {
			return result, cause
		}

		timer.Reset(req.Timeout)
		n, readErr := resp.Body.Read(buf)
		timer.Stop()
		if n > 0 {
			if m.Length >= 0 && int64(n) > m.Length-result.Bytes {
				return result, errors.New("response exceeded its declared length")
			}

			written, writeErr := f.Write(buf[:n])
			result.Bytes += int64(written)
			if writeErr != nil {
				return result, fmt.Errorf("write partial file: %w", writeErr)
			}

			if written != n {
				return result, io.ErrShortWrite
			}

			if progress != nil {
				progress(Snapshot{Bytes: result.Bytes, Total: m.Length, Resumed: result.Resumed})
			}
		}

		if cause := context.Cause(ctx); cause != nil {
			return result, cause
		}

		if errors.Is(readErr, io.EOF) {
			break
		}

		if readErr != nil {
			return result, fmt.Errorf("incomplete response: %w", readErr)
		}
	}

	if m.Length >= 0 && result.Bytes != m.Length {
		return result, fmt.Errorf("incomplete response: received %d of %d bytes", result.Bytes, m.Length)
	}

	if err := f.Finalize(); err != nil {
		return result, err
	}

	return result, nil
}
