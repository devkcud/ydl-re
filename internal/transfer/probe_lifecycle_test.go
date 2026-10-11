package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeTimeoutAndCancellation(t *testing.T) {
	for _, phase := range []string{"headers", "body", "completion", "fallback"} {
		for _, action := range []string{"timeout", "cancel"} {
			t.Run(phase+"/"+action, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				ready := make(chan struct{})
				stopped := make(chan struct{})
				var calls atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if phase != "headers" {
						if phase == "fallback" {
							w.Header().Set("Content-Length", "100")
							w.WriteHeader(http.StatusOK)
						} else {
							w.Header().Set("ETag", `"v1"`)
							w.Header().Set("Content-Range", "bytes 0-0/10")
							w.WriteHeader(http.StatusPartialContent)
							if phase == "completion" {
								fmt.Fprint(w, "a")
							}
						}

						w.(http.Flusher).Flush()
					}

					close(ready)
					<-r.Context().Done()
					close(stopped)
				}))
				defer srv.Close()
				if action == "cancel" {
					go func() {
						select {
						case <-ready:
							cancel()
						case <-ctx.Done():
						}
					}()
				}

				client := newClient(100 * time.Millisecond)
				defer client.CloseIdleConnections()
				source, _ := parseURL(srv.URL)
				result, err := probe(ctx, client, source, 100*time.Millisecond)
				if result.stream != nil {
					_, err = io.ReadAll(result.stream.Body)
					result.stream.Body.Close()
				}

				if action == "cancel" {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation error = %v", err)
					}
				} else if phase == "headers" {
					if err == nil || !strings.Contains(err.Error(), "timeout") {
						t.Fatalf("header timeout error = %v", err)
					}
				} else if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("body timeout error = %v", err)
				}

				select {
				case <-stopped:
				case <-time.After(time.Second):
					t.Fatal("failed probe left its request active")
				}

				if calls.Load() != 1 {
					t.Fatal("failed probe started fallback or retry work")
				}
			})
		}
	}
}

func TestProbeBodyOwnershipAndReadBound(t *testing.T) {
	for _, mode := range []string{"ranged", "fallback", "invalid bounds", "oversized", "failure"} {
		t.Run(mode, func(t *testing.T) {
			data := "x"
			if mode == "oversized" || mode == "fallback" {
				data = strings.Repeat("x", 1024)
			}

			body := &probeReader{reader: strings.NewReader(data)}
			var request *http.Request
			client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
				request = req
				resp := &http.Response{
					StatusCode: http.StatusPartialContent, ContentLength: -1, Request: req, Body: body,
					Header: http.Header{"Etag": {`"v1"`}, "Content-Range": {"bytes 0-0/10"}},
				}

				switch mode {
				case "fallback":
					resp.StatusCode = http.StatusOK
					resp.Header.Del("Content-Range")
				case "invalid bounds":
					resp.Header.Set("Content-Range", "bytes 0-1/10")
				case "failure":
					resp.StatusCode = http.StatusNotFound
				}

				return resp, nil
			})}
			source, _ := parseURL("http://example.org/file")
			result, err := probe(context.Background(), client, source, time.Second)
			valid := mode == "ranged" || mode == "fallback"
			if (err == nil) != valid {
				t.Fatalf("valid = %v, error = %v", valid, err)
			}

			if mode == "fallback" {
				if result.stream == nil || body.closed != 0 || body.read != 0 || request.Context().Err() != nil {
					t.Fatal("consumed or closed the caller's fallback stream")
				}

				result.stream.Body.Close()
			}

			if body.closed != 1 || body.read > 2 || !errors.Is(request.Context().Err(), context.Canceled) {
				t.Fatalf("closed = %d, read = %d, context = %v", body.closed, body.read, request.Context().Err())
			}
		})
	}
}

func TestProbeRejectsInvalidStart(t *testing.T) {
	var calls int
	client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected request")
	})}
	source, _ := parseURL("http://example.org/file")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := probe(ctx, client, source, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("early cancellation = %v", err)
	}

	for _, timeout := range []time.Duration{0, -time.Second} {
		if _, err := probe(context.Background(), client, source, timeout); err == nil {
			t.Fatal("accepted a nonpositive timeout")
		}
	}

	if calls != 0 {
		t.Fatal("invalid probe started a request")
	}
}

type probeTransport func(*http.Request) (*http.Response, error)

func (f probeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type probeReader struct {
	reader *strings.Reader
	read   int
	closed int
}

func (r *probeReader) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	r.read += n
	return n, err
}

func (r *probeReader) Close() error {
	r.closed++
	return nil
}
