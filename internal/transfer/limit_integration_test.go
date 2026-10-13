package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestRateBudgetIncludesProbesAndFailedChunks(t *testing.T) {
	for _, test := range []struct {
		saved  bool
		shared bool
	}{{}, {saved: true}, {shared: true}, {saved: true, shared: true}} {
		t.Run(fmt.Sprintf("saved=%v/shared=%v", test.saved, test.shared), func(t *testing.T) {
			saved := test.saved
			synctest.Test(t, func(t *testing.T) {
				const data = "abcdefghijklmnopqrst"
				const raw = "http://example.org/file"
				path, f, id := workerFile(t, raw, int64(len(data)), 5)
				resumed := int64(0)
				if saved {
					w, err := f.ClaimRange(ranges.Span{Start: 10, End: 15})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := w.Write([]byte(data[10:15])); err != nil {
						t.Fatal(err)
					}
					if err := w.Commit(context.Background()); err != nil {
						t.Fatal(err)
					}
					w.Close()
					resumed = 5
				}
				var probes, attempts, received atomic.Int32
				rate := int64(10)
				var shared *Budget
				if test.shared {
					shared, _ = NewBudget(rate)
					rate = 0
				}
				client := rateClient(t, rate, time.Millisecond, func(req *http.Request) (*http.Response, error) {
					start, end := requestedRange(t, req, int64(len(data)))
					resp := &http.Response{StatusCode: http.StatusPartialContent, Request: req, ContentLength: end - start + 1,
						Header: http.Header{"Etag": {`"v1"`}, "Content-Range": {fmt.Sprintf("bytes %d-%d/%d", start, end, len(data))}}}
					var reader io.Reader = strings.NewReader(data[start : end+1])
					if end == 0 {
						if probes.Add(1) == 1 {
							reader = readFunc(func(p []byte) (int, error) { return copy(p, "?"), io.ErrUnexpectedEOF })
						}
					} else if start == 0 && attempts.Add(1) == 1 {
						reader = strings.NewReader("XX")
					} else if saved && start == 10 {
						t.Error("rate-limited resume fetched a saved chunk")
					}
					resp.Body = io.NopCloser(readFunc(func(p []byte) (int, error) {
						n, err := reader.Read(p)
						received.Add(int32(n))
						return n, err
					}))
					return resp, nil
				}, shared)
				policy := retryPolicy{limit: 1, pause: func(context.Context, time.Duration) error { return nil }}
				source, _ := parseURL(raw)
				start := time.Now()
				var err error
				if saved {
					m, _ := f.Metadata()
					id, err = validateRangeResume(context.Background(), client, source, m, policy)
				} else {
					var result probeResult
					result, err = probe(context.Background(), client, source, policy)
					id = result.identity
				}
				if err != nil {
					t.Fatal(err)
				}
				last := resumed
				rollback := false
				n, err := receiveRanges(context.Background(), client, source, id, f, 4, policy, func(s Snapshot) {
					if s.Bytes < resumed || s.Bytes > int64(len(data)) || s.Resumed != resumed || s.Workers > 4 {
						t.Errorf("invalid limited progress: %+v", s)
					}
					rollback = rollback || s.Bytes < last
					last = s.Bytes
				})
				traffic := int64(len(data)) - resumed + 4
				want := time.Duration(traffic-1) * 100 * time.Millisecond
				if err != nil || n != int64(len(data)) || !rollback || probes.Load() != 2 || attempts.Load() != 2 ||
					int64(received.Load()) != traffic || time.Since(start) < want || time.Since(start) > want+time.Microsecond {
					t.Fatalf("limited retry = %d, %v, traffic=%d, probes=%d, attempts=%d, rollback=%v, elapsed=%s; want %s",
						n, err, received.Load(), probes.Load(), attempts.Load(), rollback, time.Since(start), want)
				}
				if err := f.FinalizeContext(context.Background()); err != nil {
					t.Fatal(err)
				}
				assertData(t, path, data)
			})
		})
	}
}

func TestRateBudgetSurvivesProbeFallback(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rate := int64(10)
				var shared *Budget
				if global {
					shared, _ = NewBudget(rate)
					rate = 0
				}
				client := rateClient(t, rate, time.Millisecond, func(req *http.Request) (*http.Response, error) {
					resp := &http.Response{StatusCode: http.StatusOK, Request: req, Header: make(http.Header), ContentLength: 4,
						Body: io.NopCloser(strings.NewReader("abcd"))}
					if req.Header.Get("Range") != "" {
						resp.StatusCode = http.StatusPartialContent
						resp.ContentLength = 1
						resp.Header.Set("Content-Range", "bytes 0-0/4")
						resp.Body = io.NopCloser(strings.NewReader("x"))
					}
					return resp, nil
				}, shared)
				source, _ := parseURL("http://example.org/file")
				start := time.Now()
				result, err := probe(context.Background(), client, source, retryPolicy{})
				if err != nil || result.stream == nil {
					t.Fatalf("fallback = %+v, %v", result, err)
				}
				defer result.stream.Body.Close()
				data, err := io.ReadAll(result.stream.Body)
				if err != nil || string(data) != "abcd" || time.Since(start) != 400*time.Millisecond {
					t.Fatalf("fallback budget = %q, %v, elapsed=%s", data, err, time.Since(start))
				}
			})
		})
	}
}

func TestRateWaitCancellationJoinsWorkers(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rate := int64(1)
				var shared *Budget
				if global {
					shared, _ = NewBudget(rate)
					rate = 0
				}
				const raw = "http://example.org/file"
				path, f, id := workerFile(t, raw, 20, 5)
				var reads, calls atomic.Int32
				client := rateClient(t, rate, time.Millisecond, func(req *http.Request) (*http.Response, error) {
					calls.Add(1)
					start, end := requestedRange(t, req, 20)
					return &http.Response{StatusCode: http.StatusPartialContent, Request: req, ContentLength: 5,
						Header: http.Header{"Etag": {`"v1"`}, "Content-Range": {fmt.Sprintf("bytes %d-%d/20", start, end)}},
						Body: io.NopCloser(readFunc(func(p []byte) (int, error) {
							n := copy(p, "abcde")
							reads.Add(int32(n))
							return n, nil
						}))}, nil
				}, shared)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				source, _ := parseURL(raw)
				done := make(chan error, 1)
				go func() {
					_, err := receiveRanges(ctx, client, source, id, f, 4, retryPolicy{limit: 3}, nil)
					done <- err
				}()
				synctest.Wait()
				if reads.Load() != 5 || calls.Load() != 4 {
					t.Fatalf("unbounded pending reads: bytes=%d requests=%d", reads.Load(), calls.Load())
				}
				cancel()
				if err := <-done; !errors.Is(err, context.Canceled) {
					t.Fatalf("worker cancellation = %v", err)
				}
				m, _ := f.Metadata()
				if m.Completed != 0 || calls.Load() != 4 {
					t.Fatal("canceled workers acknowledged or retried incomplete chunks")
				}
				if err := f.Close(); err != nil {
					t.Fatalf("limiter wait retained a writer: %v", err)
				}
				assertAbsent(t, path)
			})
		})
	}
}

func TestDownloadRateLimitPaths(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%v", global), func(t *testing.T) {
			for _, mode := range []string{"direct", "ignored range", "fallback", "unknown", "sequential resume", "range resume", "redirect", "retry"} {
				t.Run(mode, func(t *testing.T) {
					const data = "abcdefghijklmnopqrst"
					var traffic, attempts atomic.Int32
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("ETag", `"v1"`)
						if mode == "redirect" && r.URL.Path == "/start" {
							w.Header().Set("Location", "/end")
							w.Header().Set("Content-Length", "3")
							w.WriteHeader(http.StatusFound)
							fmt.Fprint(w, "abc")
							traffic.Add(3)
							return
						}
						body := data
						if mode == "unknown" {
							w.(http.Flusher).Flush()
						} else if mode == "sequential resume" {
							if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"v1"` {
								t.Error("limited sequential resume lost its saved identity")
							}
							w.Header().Set("Content-Range", "bytes 5-19/20")
							w.WriteHeader(http.StatusPartialContent)
							body = data[5:]
						} else if mode != "ignored range" && r.Header.Get("Range") != "" {
							start, end := requestedRange(t, r, int64(len(data)))
							w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/20", start, end))
							if mode == "fallback" {
								w.Header().Del("ETag")
							}
							w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
							w.WriteHeader(http.StatusPartialContent)
							body = data[start : end+1]
							if mode == "retry" && end > 0 && attempts.Add(1) == 1 {
								body = body[:6]
							}
						}
						traffic.Add(int32(len(body)))
						fmt.Fprint(w, body)
					}))
					defer srv.Close()
					raw := srv.URL
					if mode == "redirect" {
						raw += "/start"
					}
					path := filepath.Join(t.TempDir(), "file")
					resumed := int64(0)
					if mode == "range resume" {
						path = rangeFixture(t, raw, data, 5, []ranges.Span{{Start: 10, End: 15}})
						resumed = 5
					} else if mode == "sequential resume" {
						f, err := storage.Create(path, false)
						if err != nil {
							t.Fatal(err)
						}
						source, _ := parseURL(raw)
						if err := f.Begin(storage.Manifest{SourceID: fingerprint(source), EffectiveID: fingerprint(source),
							Length: int64(len(data)), ETag: `"v1"`, Encoding: "identity"}); err != nil {
							t.Fatal(err)
						}
						if _, err := f.Write([]byte(data[:5])); err != nil {
							t.Fatal(err)
						}
						if err := f.Close(); err != nil {
							t.Fatal(err)
						}
						resumed = 5
					}
					req := Request{URL: raw, Output: path, Limit: 100}
					if mode == "direct" || mode == "unknown" {
						req.Connections = 1
					}
					var budget *Budget
					if global {
						budget, _ = NewBudget(req.Limit)
						req.Limit = 0
					}
					start := time.Now()
					result, err := budget.Download(context.Background(), req, nil)
					want := time.Duration(traffic.Load()-10) * time.Second / 100
					if err != nil || !result.Published || result.Resumed != resumed || time.Since(start)+5*time.Millisecond < want {
						t.Fatalf("limited download = %+v, %v, elapsed=%s; minimum=%s", result, err, time.Since(start), want)
					}
					if mode == "redirect" && traffic.Load() != 27 || mode == "retry" && (traffic.Load() != 27 || attempts.Load() != 2) {
						t.Fatalf("unexpected accounting fixture: traffic=%d, attempts=%d", traffic.Load(), attempts.Load())
					}
					assertData(t, path, data)
				})
			}
		})
	}
}

func TestInvalidRateSkipsArtifacts(t *testing.T) {
	for _, req := range []Request{{Limit: -1}, {Timeout: -time.Second}} {
		req.URL = "https://example.org/file"
		req.Output = filepath.Join(t.TempDir(), "file")
		if _, err := Download(context.Background(), req, nil); err == nil {
			t.Fatal("invalid rate or timeout accepted")
		}
		assertAbsent(t, req.Output+storage.PartSuffix)
		assertAbsent(t, req.Output+storage.LockSuffix)
	}
}
