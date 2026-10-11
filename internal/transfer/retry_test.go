package transfer

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRetryBudget(t *testing.T) {
	for _, limit := range []int{0, 1, DefaultRetries, MaxRetries} {
		var attempts, pauses int
		p := retryPolicy{limit: limit, pause: func(ctx context.Context, delay time.Duration) error {
			pauses++
			if delay < retryBase/2 || delay > retryCap {
				t.Errorf("invalid delay: %s", delay)
			}
			return nil
		}}
		failure := &retryError{err: io.ErrUnexpectedEOF}
		err := p.run(context.Background(), func(attempt int) error {
			if attempt != attempts {
				t.Errorf("attempt index = %d, want %d", attempt, attempts)
			}
			attempts++
			return failure
		})
		if !errors.Is(err, io.ErrUnexpectedEOF) || attempts != limit+1 || pauses != limit {
			t.Fatalf("limit=%d: attempts=%d, pauses=%d, error=%v", limit, attempts, pauses, err)
		}
	}
}

func TestRetryStopsOnSuccessAndPermanentErrors(t *testing.T) {
	for _, final := range []error{nil, errors.New("identity changed"), &os.PathError{Op: "write", Path: "file", Err: syscall.ENOSPC}} {
		attempts := 0
		p := retryPolicy{limit: 3, pause: func(context.Context, time.Duration) error { return nil }}
		err := p.run(context.Background(), func(int) error {
			attempts++
			if attempts == 1 {
				return &retryError{err: io.ErrUnexpectedEOF}
			}
			return final
		})
		if err != final || attempts != 2 {
			t.Fatalf("attempts=%d, error=%v; want %v", attempts, err, final)
		}
	}
}

func TestRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts int
	p := retryPolicy{limit: 3, pause: func(ctx context.Context, delay time.Duration) error {
		cancel()
		return waitRetry(ctx, time.Hour)
	}}
	err := p.run(ctx, func(int) error {
		attempts++
		return &retryError{err: io.ErrUnexpectedEOF}
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("canceled retry: attempts=%d, error=%v", attempts, err)
	}

	if err := p.run(ctx, func(int) error {
		t.Fatal("started an attempt after cancellation")
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("early cancellation = %v", err)
	}
}

func TestRetryDelayBounds(t *testing.T) {
	cap := retryBase
	for attempt := range MaxRetries {
		for range 32 {
			delay := retryDelay(attempt)
			if delay < cap/2 || delay > cap {
				t.Fatalf("attempt %d delay = %s; expected [%s, %s]", attempt, delay, cap/2, cap)
			}
		}
		cap = min(2*cap, retryCap)
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		values []string
		want   time.Duration
		ok     bool
	}{
		{nil, 0, true}, {[]string{"0"}, 0, true}, {[]string{" 03 "}, 3 * time.Second, true},
		{[]string{"30"}, 30 * time.Second, true}, {[]string{"31"}, 0, false},
		{[]string{"99999999999999999999999"}, 0, false},
		{[]string{"bad"}, 0, true}, {[]string{"-1"}, 0, true}, {[]string{"1.5"}, 0, true},
		{[]string{"1", "2"}, 0, true},
		{[]string{now.Add(5 * time.Second).Format(http.TimeFormat)}, 5 * time.Second, true},
		{[]string{now.Add(-time.Second).Format(http.TimeFormat)}, 0, true},
		{[]string{now.Add(time.Hour).Format(http.TimeFormat)}, time.Hour, false},
	} {
		got, ok := retryAfter(test.values, now)
		if got != test.want || ok != test.ok {
			t.Errorf("%v: delay=%s ok=%v; want %s %v", test.values, got, ok, test.want, test.ok)
		}
	}
}

func TestRetryStatusAllowlist(t *testing.T) {
	p := retryPolicy{limit: 1}
	for _, code := range []int{408, 429, 500, 502, 503, 504} {
		resp := &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"2"}}}
		err := retryStatus(resp, time.Now())
		if !p.canRetry(context.Background(), 0, err) {
			t.Errorf("HTTP %d was not retryable: %v", code, err)
		}
		resp.Header.Set("Retry-After", "31")
		err = retryStatus(resp, time.Now())
		if err == nil || p.canRetry(context.Background(), 0, err) {
			t.Errorf("ignored excessive Retry-After: %v", err)
		}
	}
	for _, code := range []int{200, 206, 301, 400, 401, 403, 404, 412, 416, 501, 505} {
		if err := retryStatus(&http.Response{StatusCode: code}, time.Now()); err != nil {
			t.Errorf("HTTP %d unexpectedly classified for retry", code)
		}
	}
}

func TestTransientFailures(t *testing.T) {
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF, context.DeadlineExceeded, syscall.ECONNRESET,
		syscall.ECONNABORTED, syscall.ECONNREFUSED, syscall.EPIPE, &net.DNSError{IsTimeout: true}} {
		if !transient(err) {
			t.Errorf("not retryable: %v", err)
		}
	}
	for _, err := range []error{context.Canceled, errRedirectLimit, errRedirectTarget, errDowngrade,
		&tls.CertificateVerificationError{Err: errors.New("untrusted")}, &net.DNSError{IsNotFound: true},
		syscall.ENOSPC, errors.New("malformed protocol")} {
		if transient(err) {
			t.Errorf("unexpected retry: %v", err)
		}
	}
}

func TestRequestRetriesCloseResponsesAndKeepConditions(t *testing.T) {
	var attempts int
	var bodies []*probeReader
	client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
		attempts++
		if req.Header.Get("Range") != "bytes=4-7" || req.Header.Get("If-Range") != `"v1"` {
			t.Error("retry lost request conditions")
		}
		req.Header.Set("If-Range", `"mutated"`)
		body := &probeReader{reader: strings.NewReader("body")}
		bodies = append(bodies, body)
		code := http.StatusServiceUnavailable
		if attempts == 3 {
			code = http.StatusPartialContent
		}
		return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"2"}}, Body: body, Request: req}, nil
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://example.org?token=private-secret", nil)
	req.Header.Set("Range", "bytes=4-7")
	req.Header.Set("If-Range", `"v1"`)
	p := retryPolicy{limit: 3, pause: func(ctx context.Context, delay time.Duration) error {
		if delay < 2*time.Second || delay > maxRetryAfter {
			t.Errorf("did not honor Retry-After: %s", delay)
		}
		return nil
	}}
	resp, err := p.request(context.Background(), client, req)
	if err != nil || attempts != 3 || resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("attempts=%d, error=%v", attempts, err)
	}
	defer resp.Body.Close()
	if bodies[0].closed != 1 || bodies[1].closed != 1 || bodies[2].closed != 0 || req.Header.Get("If-Range") != `"v1"` {
		t.Fatal("retry leaked a response, closed the result, or mutated the original request")
	}
}

func TestRequestRetryErrorsAreRedacted(t *testing.T) {
	client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: syscall.ECONNRESET}
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://example.org?token=private-secret", nil)
	_, err := requestOnce(context.Background(), client, req)
	var retry *retryError
	if !errors.As(err, &retry) || strings.Contains(err.Error(), "private-secret") || strings.Contains(err.Error(), "https://") {
		t.Fatalf("unsafe or unclassified request error: %v", err)
	}
}

func TestBodyFailureClassification(t *testing.T) {
	for _, test := range []struct {
		err   error
		want  error
		retry bool
	}{
		{io.ErrUnexpectedEOF, io.ErrUnexpectedEOF, true},
		{&url.Error{Op: "Read", URL: "https://example.org?token=private-secret", Err: syscall.ECONNRESET}, io.ErrUnexpectedEOF, true},
		{context.DeadlineExceeded, context.DeadlineExceeded, true},
		{context.Canceled, context.Canceled, false},
		{errors.New("private-secret malformed chunk"), nil, false},
	} {
		err := bodyFailure(test.err)
		var retry *retryError
		if errors.As(err, &retry) != test.retry || strings.Contains(err.Error(), "private-secret") ||
			(test.want != nil && !errors.Is(err, test.want)) {
			t.Errorf("body classification = %v, want %v (retry=%v)", err, test.want, test.retry)
		}
	}
}
