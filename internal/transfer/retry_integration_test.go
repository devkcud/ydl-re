package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestRetryReplacesWholeChunkWithoutDuplicateProgress(t *testing.T) {
	for _, limit := range []int{1, 2} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			var attempts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start, end := requestedRange(t, r, 12)
				if r.Header.Get("If-Range") != `"v1"` {
					t.Error("retry lost the representation validator")
				}
				if start == 0 {
					switch attempts.Add(1) {
					case 1:
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					case 2:
						w.Header().Set("ETag", `"v1"`)
						w.Header().Set("Content-Range", "bytes 0-3/12")
						w.WriteHeader(http.StatusPartialContent)
						fmt.Fprint(w, "XXXX")
						w.(http.Flusher).Flush()
						panic(http.ErrAbortHandler)
					}
				}
				if start == 4 {
					t.Error("re-downloaded a checkpointed chunk")
				}
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/12", start, end))
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, "abcdefghijkl"[start:end+1])
			}))
			defer srv.Close()
			path, f, id := workerFile(t, srv.URL, 12, 4)
			w, err := f.ClaimRange(ranges.Span{Start: 4, End: 8})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("efgh")); err != nil {
				t.Fatal(err)
			}
			if err := w.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			w.Close()
			pauses := 0
			policy := retryPolicy{limit: limit, pause: func(ctx context.Context, delay time.Duration) error {
				pauses++
				m, _ := f.Metadata()
				if m.Completed != 4 || len(m.Ranges) != 1 {
					t.Error("failed attempt advanced the checkpoint")
				}
				claim, err := f.ClaimRange(ranges.Span{End: 4})
				if err != nil {
					t.Errorf("old writer survived into backoff: %v", err)
					return err
				}
				return claim.Close()
			}}
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			var snapshots []Snapshot
			n, err := receiveRanges(context.Background(), client, source, id, f, 1, policy, func(s Snapshot) {
				snapshots = append(snapshots, s)
			})
			if attempts.Load() != int32(limit+1) || pauses != limit {
				t.Fatalf("budget multiplied: attempts=%d pauses=%d", attempts.Load(), pauses)
			}
			for _, s := range snapshots {
				if s.Bytes < 4 || s.Bytes > 12 || s.Resumed != 4 || s.Workers < 0 || s.Workers > 1 {
					t.Fatalf("incorrect retry progress: %+v", s)
				}
			}
			if limit == 1 {
				if err == nil {
					t.Fatal("exceeded the shared header/body retry budget")
				}
				m, _ := f.Metadata()
				if m.Completed != 4 {
					t.Fatal("exhausted attempt became completed coverage")
				}
				assertAbsent(t, path)
				return
			}
			if err != nil || n != 12 {
				t.Fatalf("retried transfer = %d, %v", n, err)
			}
			rollback := false
			for i := 1; i < len(snapshots); i++ {
				rollback = rollback || snapshots[i].Bytes < snapshots[i-1].Bytes
			}
			if !rollback {
				t.Fatal("failed bytes were counted again during replacement")
			}
			if err := f.FinalizeContext(context.Background()); err != nil {
				t.Fatal(err)
			}
			assertData(t, path, "abcdefghijkl")
		})
	}
}

func TestRetryStopsOnChangedOrInvalidRanges(t *testing.T) {
	for _, mode := range []string{"etag", "bounds", "encoding", "oversized", "malformed", "storage"} {
		t.Run(mode, func(t *testing.T) {
			var attempts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				if mode == "malformed" {
					conn, buf, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					defer conn.Close()
					fmt.Fprint(buf, "HTTP/1.1 206 Partial Content\r\nETag: \"v1\"\r\nContent-Range: bytes 0-3/4\r\nTransfer-Encoding: chunked\r\n\r\nnot-hex\r\n")
					buf.Flush()
					return
				}
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", "bytes 0-3/4")
				switch mode {
				case "etag":
					w.Header().Set("ETag", `"v2"`)
				case "bounds":
					w.Header().Set("Content-Range", "bytes 1-3/4")
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				}
				w.WriteHeader(http.StatusPartialContent)
				w.(http.Flusher).Flush()
				fmt.Fprint(w, "abcd")
				if mode == "oversized" {
					fmt.Fprint(w, "x")
				}
			}))
			defer srv.Close()
			path, f, id := workerFile(t, srv.URL, 4, 4)
			policy := retryPolicy{limit: 3, pause: func(context.Context, time.Duration) error {
				if mode == "storage" {
					if err := os.Rename(path+storage.PartSuffix, path+".old"); err != nil {
						return err
					}
					return os.WriteFile(path+storage.PartSuffix, []byte("keep"), 0o600)
				}
				return nil
			}}
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			if _, err := receiveRanges(context.Background(), client, source, id, f, 1, policy, nil); err == nil || attempts.Load() != 2 {
				t.Fatalf("terminal failure retried: attempts=%d, error=%v", attempts.Load(), err)
			}
			m, _ := f.Metadata()
			if m.Completed != 0 || m.State != storage.StatePartial {
				t.Fatal("invalid bytes advanced coverage")
			}
			if mode == "storage" {
				assertData(t, path+storage.PartSuffix, "keep")
			}
			assertAbsent(t, path)
		})
	}
}

func TestRetryCancellationKeepsWorkersBounded(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, f, id := workerFile(t, srv.URL, 16, 4)
	client := newClient(time.Second, nil)
	defer client.CloseIdleConnections()
	source, _ := parseURL(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	paused := make(chan struct{}, 2)
	policy := retryPolicy{limit: 3, pause: func(ctx context.Context, delay time.Duration) error {
		if delay != maxRetryAfter {
			t.Errorf("Retry-After delay = %s", delay)
		}
		select {
		case paused <- struct{}{}:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
		return waitRetry(ctx, delay)
	}}
	done := make(chan error, 1)
	go func() {
		_, err := receiveRanges(ctx, client, source, id, f, 2, policy, nil)
		done <- err
	}()
	for range 2 {
		select {
		case <-paused:
		case <-time.After(3 * time.Second):
			t.Fatal("workers did not enter backoff")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || attempts.Load() != 2 {
			t.Fatalf("cancellation = %v, attempts=%d", err, attempts.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt backoff")
	}
	if err := f.Close(); err != nil {
		t.Fatalf("backoff retained a writer: %v", err)
	}
}

func TestProbeRetriesShareRequestAndBodyBudget(t *testing.T) {
	for _, test := range []struct {
		saved bool
		limit int
	}{{false, 1}, {true, 1}, {false, 2}, {true, 2}} {
		t.Run(fmt.Sprintf("%v/%d", test.saved, test.limit), func(t *testing.T) {
			var attempts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := attempts.Add(1)
				if n == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", "bytes 0-0/8")
				w.Header().Set("Content-Length", "1")
				w.WriteHeader(http.StatusPartialContent)
				if n > 2 {
					fmt.Fprint(w, "a")
				}
			}))
			defer srv.Close()
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			policy := retryPolicy{limit: test.limit, pause: func(context.Context, time.Duration) error { return nil }}
			var err error
			if test.saved {
				path, f, _ := workerFile(t, srv.URL, 8, 4)
				before, _ := os.ReadFile(path + storage.ManifestSuffix)
				m, _ := f.Metadata()
				_, err = validateRangeResume(context.Background(), client, source, m, policy)
				after, _ := os.ReadFile(path + storage.ManifestSuffix)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("failed validation changed saved metadata")
				}
			} else {
				_, err = probe(context.Background(), client, source, policy)
			}
			if (err == nil) != (test.limit == 2) || attempts.Load() != int32(test.limit+1) {
				t.Fatalf("probe budget multiplied: attempts=%d, error=%v", attempts.Load(), err)
			}
		})
	}
}

func TestRetryIdleRangeTimeoutUsesNewContext(t *testing.T) {
	var attempts atomic.Int32
	stopped := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", "bytes 0-3/4")
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusPartialContent)
		if n == 1 {
			fmt.Fprint(w, "?")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(stopped)
			return
		}
		fmt.Fprint(w, "abcd")
	}))
	defer srv.Close()
	path, f, id := workerFile(t, srv.URL, 4, 4)
	client := newClient(50*time.Millisecond, nil)
	defer client.CloseIdleConnections()
	source, _ := parseURL(srv.URL)
	policy := retryPolicy{limit: 1, pause: func(ctx context.Context, delay time.Duration) error {
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Error("timed-out request was not closed before backoff")
		}
		return ctx.Err()
	}}
	n, err := receiveRanges(context.Background(), client, source, id, f, 1, policy, nil)
	if err != nil || n != 4 || attempts.Load() != 2 {
		t.Fatalf("timeout retry = %d, %v, attempts=%d", n, err, attempts.Load())
	}
	if err := f.Finalize(); err != nil {
		t.Fatal(err)
	}
	assertData(t, path, "abcd")
}

func TestRetryDiscardClosesActiveResponse(t *testing.T) {
	var attempts atomic.Int32
	stopped := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(stopped)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := newClient(time.Second, nil)
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	policy := retryPolicy{limit: 1, pause: func(context.Context, time.Duration) error {
		select {
		case <-stopped:
			return nil
		case <-time.After(time.Second):
			return errors.New("failed response was not closed")
		}
	}}
	resp, err := policy.request(ctx, client, req)
	if err != nil || attempts.Load() != 2 {
		t.Fatalf("response cleanup: attempts=%d, error=%v", attempts.Load(), err)
	}
	resp.Body.Close()
}

func TestFallbackRetriesDoNotRepeatProbe(t *testing.T) {
	var probes, full atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			probes.Add(1)
			w.Header().Set("Content-Range", "bytes 0-0/8")
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, "x")
			return
		}
		if full.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "fallback")
	}))
	defer srv.Close()
	client := newClient(time.Second, nil)
	defer client.CloseIdleConnections()
	source, _ := parseURL(srv.URL)
	policy := retryPolicy{limit: 1, pause: func(context.Context, time.Duration) error { return nil }}
	result, err := probe(context.Background(), client, source, policy)
	if err != nil || result.stream == nil {
		t.Fatalf("fallback = %+v, %v", result, err)
	}
	defer result.stream.Body.Close()
	data, err := io.ReadAll(result.stream.Body)
	if err != nil || string(data) != "fallback" || probes.Load() != 1 || full.Load() != 2 {
		t.Fatalf("body=%q, error=%v, probes=%d full requests=%d", data, err, probes.Load(), full.Load())
	}
}

func TestSequentialBodyDoesNotRestartAutomatically(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Length", "20")
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, "partial")
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Connections: 1}, nil)
	if err == nil || result.Published || attempts.Load() != 1 {
		t.Fatalf("sequential body restarted: %+v, %v, attempts=%d", result, err, attempts.Load())
	}
	assertData(t, path+storage.PartSuffix, "partial")
}

func TestSequentialResumeRetriesBeforeReusingBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=3-" || r.Header.Get("If-Range") != `"v1"` {
			t.Error("sequential retry lost its saved offset or validator")
		}
		partial, err := os.ReadFile(path + storage.PartSuffix)
		if err != nil || string(partial) != "abcBAD" {
			t.Error("retry changed the unconfirmed tail before validation")
		}
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", "bytes 3-7/8")
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, "defgh")
	}))
	defer srv.Close()
	source, _ := parseURL(srv.URL)
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Begin(storage.Manifest{SourceID: fingerprint(source), EffectiveID: fingerprint(source),
		Length: 8, ETag: `"v1"`, Encoding: "identity"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+storage.PartSuffix, []byte("abcBAD"), 0o600); err != nil {
		t.Fatal(err)
	}
	limit := 1
	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Resume: true, Retries: &limit}, nil)
	if err != nil || !result.Published || result.Resumed != 3 || attempts.Load() != 2 {
		t.Fatalf("retried sequential resume = %+v, %v, attempts=%d", result, err, attempts.Load())
	}
	assertData(t, path, "abcdefgh")
}

func TestInvalidRetriesSkipArtifacts(t *testing.T) {
	for _, limit := range []int{-1, MaxRetries + 1} {
		path := filepath.Join(t.TempDir(), "file")
		_, err := Download(context.Background(), Request{URL: "https://example.org/file", Output: path, Retries: &limit}, nil)
		if err == nil {
			t.Fatalf("accepted invalid retry limit %d", limit)
		}
		assertAbsent(t, path+storage.PartSuffix)
		assertAbsent(t, path+storage.LockSuffix)
	}
}
