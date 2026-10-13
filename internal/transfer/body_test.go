package transfer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestBodyPacingExcludesIdleTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader := strings.NewReader("abcd")
		client := rateClient(t, 10, 10*time.Millisecond, func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
				Body: io.NopCloser(readFunc(func(p []byte) (int, error) {
					if len(p) > 1 {
						t.Error("body read exceeded the one-byte burst")
					}
					return reader.Read(p)
				}))}, nil
		})
		start := time.Now()
		resp, err := client.Get("http://example.org/file")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil || string(data) != "abcd" || time.Since(start) != 300*time.Millisecond {
			t.Fatalf("paced body = %q, %v, elapsed=%s", data, err, time.Since(start))
		}
	})
}

func TestBodyTimeoutDoesNotCancelLaterAttempts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := rateClient(t, 10, 10*time.Millisecond, func(req *http.Request) (*http.Response, error) {
			calls++
			var body io.Reader = strings.NewReader("ok")
			if calls == 1 {
				body = readFunc(func([]byte) (int, error) {
					<-req.Context().Done()
					return 0, req.Context().Err()
				})
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req, Body: io.NopCloser(body)}, nil
		})
		for attempt := range 2 {
			resp, err := client.Get("http://example.org/file")
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if attempt == 0 && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("idle body did not time out: %v", err)
			}
			if attempt == 1 && (err != nil || string(data) != "ok") {
				t.Fatalf("later attempt = %q, %v", data, err)
			}
		}
	})
}

func TestBodyCancellationInterruptsRateWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		body := &probeReader{reader: strings.NewReader("abc")}
		var sent *http.Request
		client := rateClient(t, 1, time.Millisecond, func(req *http.Request) (*http.Response, error) {
			sent = req
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req, Body: body}, nil
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.org/file", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 10)
		if n, err := resp.Body.Read(buf); n != 1 || err != nil {
			t.Fatalf("initial burst = %d, %v", n, err)
		}
		done := make(chan error, 1)
		go func() {
			n, err := resp.Body.Read(buf)
			if n != 0 {
				t.Error("canceled wait delivered pending bytes")
			}
			done <- err
		}()
		synctest.Wait()
		if body.read != 2 {
			t.Fatalf("pending read = %d bytes", body.read)
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled wait = %v", err)
		}
		resp.Body.Close()
		if body.closed != 1 || sent.Context().Err() != context.Canceled {
			t.Fatal("canceled body retained request ownership")
		}
	})
}

func TestBodyBudgetIncludesRedirectReads(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(strconv.FormatBool(global), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rate := int64(10)
				var shared *Budget
				if global {
					shared, _ = NewBudget(rate)
					rate = 0
				}
				calls := 0
				client := rateClient(t, rate, time.Millisecond, func(req *http.Request) (*http.Response, error) {
					calls++
					resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
						Body: io.NopCloser(strings.NewReader("de")), ContentLength: 2}
					if req.URL.Path == "/start" {
						resp.StatusCode = http.StatusFound
						resp.Header.Set("Location", "/end")
						resp.Body = io.NopCloser(strings.NewReader("abc"))
						resp.ContentLength = 3
					}
					return resp, nil
				}, shared)
				start := time.Now()
				resp, err := client.Get("http://example.org/start")
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err != nil || string(data) != "de" || calls != 2 || time.Since(start) != 400*time.Millisecond {
					t.Fatalf("redirect body = %q, %v, requests=%d, elapsed=%s", data, err, calls, time.Since(start))
				}
			})
		})
	}
}

func rateClient(t *testing.T, rate int64, timeout time.Duration, next probeTransport, shared ...*Budget) *http.Client {
	t.Helper()
	limit, err := NewBudget(rate)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: &bodyTransport{next: next, timeout: timeout, budgets: append([]*Budget{limit}, shared...)}, CheckRedirect: checkRedirect}
}

type readFunc func([]byte) (int, error)

func (f readFunc) Read(p []byte) (int, error) {
	return f(p)
}
