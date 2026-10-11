package transfer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type bodyTransport struct {
	next    http.RoundTripper
	timeout time.Duration
	limit   *limiter
}

func (t *bodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(req.Context())
	resp, err := t.next.RoundTrip(req.Clone(ctx))
	if err != nil {
		cancel(nil)
		return resp, err
	}

	resp.Body = &responseBody{ReadCloser: resp.Body, ctx: ctx, cancel: cancel, timeout: t.timeout, limit: t.limit}
	return resp, nil
}

func (t *bodyTransport) CloseIdleConnections() {
	if closer, ok := t.next.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type responseBody struct {
	io.ReadCloser
	ctx     context.Context
	cancel  context.CancelCauseFunc
	timeout time.Duration
	limit   *limiter
}

func (b *responseBody) Read(data []byte) (int, error) {
	if cause := context.Cause(b.ctx); cause != nil {
		return 0, cause
	}
	if b.limit != nil {
		data = data[:min(len(data), b.limit.burst)]
	}

	timer := time.AfterFunc(b.timeout, func() {
		b.cancel(fmt.Errorf("response body idle for %s: %w", b.timeout, context.DeadlineExceeded))
	})
	n, err := b.ReadCloser.Read(data)
	timer.Stop()

	if waitErr := b.limit.wait(b.ctx, n); waitErr != nil {
		return 0, waitErr
	}

	return n, err
}

func (b *responseBody) Close() error {
	b.cancel(nil)
	return b.ReadCloser.Close()
}
