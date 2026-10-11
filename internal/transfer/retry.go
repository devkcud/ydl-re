package transfer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const DefaultRetries = 3
const MaxRetries = 10
const retryBase = 250 * time.Millisecond
const retryCap = 8 * time.Second
const maxRetryAfter = 30 * time.Second

type retryError struct {
	err   error
	after time.Duration
}

func (e *retryError) Error() string {
	return e.err.Error()
}

func (e *retryError) Unwrap() error {
	return e.err
}

type retryPolicy struct {
	limit int
	pause func(context.Context, time.Duration) error
}

func (p retryPolicy) canRetry(ctx context.Context, attempt int, err error) bool {
	var retry *retryError
	return ctx.Err() == nil && attempt < p.limit && errors.As(err, &retry)
}

func (p retryPolicy) run(ctx context.Context, run func(int) error) error {
	if p.limit < 0 || p.limit > MaxRetries {
		return fmt.Errorf("retries must be between 0 and %d", MaxRetries)
	}

	for attempt := 0; ; attempt++ {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}

		err := run(attempt)
		if err == nil {
			return nil
		}

		if cause := context.Cause(ctx); cause != nil {
			return cause
		}

		if !p.canRetry(ctx, attempt, err) {
			return err
		}

		var retry *retryError
		errors.As(err, &retry)
		delay := max(retryDelay(attempt), retry.after)
		pause := p.pause
		if pause == nil {
			pause = waitRetry
		}

		if err := pause(ctx, delay); err != nil {
			return err
		}
	}
}

func retryDelay(attempt int) time.Duration {
	delay := retryBase
	for i := 0; i < attempt && delay < retryCap; i++ {
		delay = min(2*delay, retryCap)
	}

	return delay/2 + time.Duration(rand.Int64N(int64(delay/2)+1))
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return context.Cause(ctx)
	}
}

func (p retryPolicy) request(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := p.run(ctx, func(int) error {
		var err error
		resp, err = requestOnce(ctx, client, req)
		return err
	})
	return resp, err
}

func requestOnce(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	if cause := context.Cause(ctx); cause != nil {
		return nil, cause
	}

	resp, err := client.Do(req.Clone(ctx))
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return nil, cause
		}

		safe := requestError(err)
		if transient(err) {
			return nil, &retryError{err: safe}
		}

		return nil, safe
	}

	if err := retryStatus(resp, time.Now()); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func retryStatus(resp *http.Response, now time.Time) error {
	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
	default:
		return nil
	}

	err := fmt.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	after, ok := retryAfter(resp.Header.Values("Retry-After"), now)
	if !ok {
		return fmt.Errorf("%w; Retry-After exceeds %s", err, maxRetryAfter)
	}

	return &retryError{err: err, after: after}
}

func retryAfter(values []string, now time.Time) (time.Duration, bool) {
	if len(values) != 1 {
		return 0, true
	}

	value := strings.TrimSpace(values[0])
	if value != "" && strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) < 0 {
		seconds, err := strconv.ParseUint(value, 10, 64)
		if err != nil || seconds > uint64(maxRetryAfter/time.Second) {
			return 0, false
		}

		return time.Duration(seconds) * time.Second, true
	}

	date, err := http.ParseTime(value)
	if err != nil {
		return 0, true
	}

	delay := max(time.Duration(0), date.Sub(now))
	return delay, delay <= maxRetryAfter
}

func transient(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	for _, known := range []error{errRedirectLimit, errRedirectTarget, errDowngrade} {
		if errors.Is(err, known) {
			return false
		}
	}

	var cert *tls.CertificateVerificationError
	if errors.As(err, &cert) {
		return false
	}

	var dns *net.DNSError
	if errors.As(err, &dns) && dns.IsNotFound {
		return false
	}

	var network net.Error
	if errors.As(err, &network) && (network.Timeout() || network.Temporary()) {
		return true
	}

	for _, known := range []error{io.EOF, io.ErrUnexpectedEOF, syscall.ECONNRESET, syscall.ECONNABORTED,
		syscall.ECONNREFUSED, syscall.EPIPE, syscall.ETIMEDOUT} {
		if errors.Is(err, known) {
			return true
		}
	}

	return false
}

func bodyFailure(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}

	if transient(err) {
		var network net.Error
		if errors.As(err, &network) && network.Timeout() {
			return &retryError{err: fmt.Errorf("response body timeout: %w", context.DeadlineExceeded)}
		}

		return &retryError{err: fmt.Errorf("incomplete response body: %w", io.ErrUnexpectedEOF)}
	}

	return errors.New("invalid response body")
}
