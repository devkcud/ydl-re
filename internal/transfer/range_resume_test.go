package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestRangeResume(t *testing.T) {
	cases := []struct {
		name     string
		data     string
		size     int64
		done     []ranges.Span
		count    int
		explicit bool
		chunked  bool
	}{
		{"automatic", "abcdefghijklmnopq", 4, []ranges.Span{{Start: 0, End: 4}, {Start: 8, End: 12}}, 0, false, false},
		{"explicit", "abcdefghijklmnopq", 4, []ranges.Span{{Start: 4, End: 8}, {Start: 16, End: 17}}, 2, true, false},
		{"one worker", "abcdefghijklmnopq", 4, []ranges.Span{{Start: 4, End: 8}, {Start: 8, End: 12}}, 1, false, false},
		{"chunked", "abcdefghijklmnopq", 4, []ranges.Span{{Start: 4, End: 8}}, 3, true, true},
		{"no saved chunks", "abcdefghijklmnopq", 4, nil, 2, true, false},
		{"all saved", "abcde", 4, []ranges.Span{{Start: 0, End: 4}, {Start: 4, End: 5}}, 2, true, false},
		{"one byte", "a", 1, nil, 2, true, false},
		{"empty", "", 4, nil, 1, true, false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			var mu sync.Mutex
			var requests []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first := calls.Add(1) == 1
				if r.Header.Get("Accept-Encoding") != "identity" {
					t.Error("resume did not request identity encoding")
				}
				w.Header().Set("ETag", `"v1"`)
				if len(test.data) == 0 {
					if !first || r.Header.Get("Range") != "" || r.Header.Get("If-Match") != `"v1"` {
						t.Error("empty checkpoint did not use a conditional full request")
					}
					w.Header().Set("Content-Length", "0")
					return
				}

				if r.Header.Get("If-Range") != `"v1"` || (first && r.Header.Get("Range") != "bytes=0-0") {
					t.Error("resume did not preserve the saved validator or preflight bounds")
				}
				start, end := requestedRange(t, r, int64(len(test.data)))
				if !first {
					mu.Lock()
					requests = append(requests, r.Header.Get("Range"))
					mu.Unlock()
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(test.data)))
				if !test.chunked {
					w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
				}
				w.WriteHeader(http.StatusPartialContent)
				if test.chunked {
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, test.data[start:end+1])
			}))
			defer srv.Close()
			path := rangeFixture(t, srv.URL, test.data, test.size, test.done)
			saved, err := storage.Inspect(path)
			if err != nil {
				t.Fatal(err)
			}
			var snapshots []Snapshot
			result, err := Download(context.Background(), Request{
				URL: srv.URL, Output: path, Connections: test.count, Resume: test.explicit,
			}, func(s Snapshot) { snapshots = append(snapshots, s) })
			if err != nil || !result.Published || result.Bytes != int64(len(test.data)) || result.Resumed != saved.Completed {
				t.Fatalf("range resume = %+v, error = %v", result, err)
			}

			missing, err := ranges.Missing(saved.Length, saved.ChunkSize, saved.Spans())
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, span := range missing {
				value, _ := span.HTTPRange()
				want = append(want, value)
			}
			sort.Strings(want)
			mu.Lock()
			sort.Strings(requests)
			if !reflect.DeepEqual(requests, want) || calls.Load() != int32(len(want)+1) {
				t.Errorf("requested %v, want %v after one preflight", requests, want)
			}
			mu.Unlock()
			if len(snapshots) == 0 || snapshots[0].Bytes != saved.Completed || snapshots[0].Workers != 0 {
				t.Fatalf("incorrect initial progress: %+v", snapshots)
			}
			for _, s := range snapshots {
				if s.Resumed != saved.Completed || s.Bytes < saved.Completed || s.Bytes > int64(len(test.data)) {
					t.Fatalf("saved bytes or probe traffic counted incorrectly: %+v", s)
				}
			}
			assertData(t, path, test.data)
			assertAbsent(t, path+storage.PartSuffix)
			assertAbsent(t, path+storage.ManifestSuffix)
		})
	}
}

func TestRangeResumePreflightRefusalPreservesArtifacts(t *testing.T) {
	for _, mode := range []string{
		"etag", "weak", "missing etag", "duplicate etag", "encoding", "start", "end", "total", "unknown total",
		"missing range", "duplicate range", "overflow", "length", "short", "oversized", "broken", "redirect",
		"ignored", "412", "416", "worker version", "full coverage", "timeout",
	} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if mode == "416" && n == 2 {
					if r.Header.Get("Range") != "" || r.Header.Get("If-Range") != "" {
						t.Error("416 re-probe retained rejected range conditions")
					}
					fmt.Fprint(w, "new whole representation")
					return
				}
				if mode == "redirect" && r.URL.Path != "/changed" {
					http.Redirect(w, r, "/changed", http.StatusFound)
					return
				}
				start, end := requestedRange(t, r, 12)
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/12", start, end))
				switch mode {
				case "etag", "full coverage":
					w.Header().Set("ETag", `"changed"`)
				case "worker version":
					if n > 1 {
						w.Header().Set("ETag", `"changed"`)
					}
				case "weak":
					w.Header().Set("ETag", `W/"v1"`)
				case "missing etag":
					w.Header().Del("ETag")
				case "duplicate etag":
					w.Header().Add("ETag", `"v1"`)
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				case "start":
					w.Header().Set("Content-Range", "bytes 1-1/12")
				case "end":
					w.Header().Set("Content-Range", "bytes 0-1/12")
				case "total":
					w.Header().Set("Content-Range", "bytes 0-0/13")
				case "unknown total":
					w.Header().Set("Content-Range", "bytes 0-0/*")
				case "missing range":
					w.Header().Del("Content-Range")
				case "duplicate range":
					w.Header().Add("Content-Range", "bytes 0-0/12")
				case "overflow":
					w.Header().Set("Content-Range", "bytes 0-0/9223372036854775808")
				case "length":
					w.Header().Set("Content-Length", "2")
				case "short":
					w.Header().Set("Content-Length", "1")
				case "ignored":
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, "abcdefghijkl")
					return
				case "412":
					w.WriteHeader(http.StatusPreconditionFailed)
					return
				case "416":
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				w.WriteHeader(http.StatusPartialContent)
				w.(http.Flusher).Flush()
				if mode == "short" {
					return
				}
				if mode == "timeout" {
					<-r.Context().Done()
					return
				}
				fmt.Fprint(w, "abcdefghijkl"[start:end+1])
				if mode == "oversized" {
					fmt.Fprint(w, "x")
				} else if mode == "broken" {
					w.(http.Flusher).Flush()
					panic(http.ErrAbortHandler)
				}
			}))
			defer srv.Close()
			url := srv.URL + "?token=private-secret"
			done := []ranges.Span{{Start: 4, End: 8}}
			if mode == "full coverage" {
				done = []ranges.Span{{Start: 0, End: 4}, {Start: 4, End: 8}, {Start: 8, End: 12}}
			}
			path := rangeFixture(t, url, "abcdefghijkl", 4, done)
			before, meta := rangeArtifacts(t, path)
			result, err := Download(context.Background(), Request{
				URL: url, Output: path, Resume: true, Overwrite: true, Connections: 1, Timeout: 100 * time.Millisecond,
			}, nil)
			wantCalls := int32(1)
			if mode == "416" || mode == "redirect" || mode == "worker version" {
				wantCalls = 2
			}
			if err == nil || strings.Contains(err.Error(), "private-secret") || result.Published || calls.Load() != wantCalls {
				t.Fatalf("refusal = %+v, error = %v, calls = %d", result, err, calls.Load())
			}
			if mode == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("timeout error = %v", err)
			}
			after, saved := rangeArtifacts(t, path)
			if !bytes.Equal(before, after) || !bytes.Equal(meta, saved) {
				t.Fatal("refused range resume changed saved artifacts")
			}
			assertAbsent(t, path)
		})
	}
}

func TestRangeResumeLocalRefusalSkipsNetwork(t *testing.T) {
	for _, mode := range []string{"source", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
			}))
			defer srv.Close()
			url := srv.URL + "?token=original-secret"
			path := rangeFixture(t, url, "abcdefghijkl", 4, []ranges.Span{{Start: 4, End: 8}})
			if mode == "source" {
				url = srv.URL + "?token=replacement-secret"
			} else if err := os.WriteFile(path+storage.PartSuffix, []byte("????BAD!????"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, meta := rangeArtifacts(t, path)
			_, err := Download(context.Background(), Request{URL: url, Output: path, Resume: true}, nil)
			if err == nil || calls.Load() != 0 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("local refusal = %v, requests = %d", err, calls.Load())
			}
			after, saved := rangeArtifacts(t, path)
			if !bytes.Equal(before, after) || !bytes.Equal(meta, saved) {
				t.Fatal("local refusal changed evidence")
			}
		})
	}
}

func TestRangeResumeCancellationPreservesArtifacts(t *testing.T) {
	for _, phase := range []string{"preflight", "before workers"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ready := make(chan struct{})
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Range") != "bytes=0-0" {
					t.Error("started a worker after cancellation")
				}
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", "bytes 0-0/12")
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, "a")
				if phase == "preflight" {
					w.(http.Flusher).Flush()
					close(ready)
					<-r.Context().Done()
				}
			}))
			defer srv.Close()
			if phase == "preflight" {
				go func() {
					select {
					case <-ready:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			path := rangeFixture(t, srv.URL, "abcdefghijkl", 4, []ranges.Span{{Start: 4, End: 8}})
			before, meta := rangeArtifacts(t, path)
			result, err := Download(ctx, Request{URL: srv.URL, Output: path}, func(s Snapshot) {
				if phase == "before workers" {
					cancel()
				}
			})
			if !errors.Is(err, context.Canceled) || result.Published || calls.Load() != 1 {
				t.Fatalf("cancellation = %+v, %v, requests = %d", result, err, calls.Load())
			}
			after, saved := rangeArtifacts(t, path)
			if !bytes.Equal(before, after) || !bytes.Equal(meta, saved) {
				t.Fatal("canceled validation changed saved artifacts")
			}
		})
	}
}

func TestRangeResumeInterruptedThenCompleted(t *testing.T) {
	for _, mode := range []string{"short", "cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			const data = "abcdefghijkl"
			var fail atomic.Bool
			fail.Store(true)
			var mu sync.Mutex
			var requests []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Header.Get("Range"))
				mu.Unlock()
				start, end := requestedRange(t, r, int64(len(data)))
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
				w.WriteHeader(http.StatusPartialContent)
				if end > 0 && fail.CompareAndSwap(true, false) {
					fmt.Fprint(w, data[start:start+1])
					w.(http.Flusher).Flush()
					if mode != "short" {
						<-r.Context().Done()
					}
					return
				}
				fmt.Fprint(w, data[start:end+1])
			}))
			defer srv.Close()
			path := rangeFixture(t, srv.URL, data, 4, []ranges.Span{{Start: 4, End: 8}})
			_, meta := rangeArtifacts(t, path)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := Download(ctx, Request{URL: srv.URL, Output: path, Connections: 1, Timeout: 100 * time.Millisecond}, func(s Snapshot) {
				if mode == "cancel" && s.Bytes > s.Resumed {
					cancel()
				}
			})
			if err == nil || result.Published || result.Resumed != 4 ||
				(mode == "cancel" && !errors.Is(err, context.Canceled)) ||
				(mode == "timeout" && !errors.Is(err, context.DeadlineExceeded)) {
				t.Fatalf("interrupted resume = %+v, error = %v", result, err)
			}
			partial, saved := rangeArtifacts(t, path)
			if !bytes.Equal(meta, saved) || string(partial[4:8]) != data[4:8] {
				t.Fatal("failed attempt changed a completed chunk or checkpoint")
			}
			mu.Lock()
			requests = nil
			mu.Unlock()
			result, err = Download(context.Background(), Request{URL: srv.URL, Output: path, Resume: true}, nil)
			if err != nil || !result.Published || result.Resumed != 4 || result.Bytes != int64(len(data)) {
				t.Fatalf("second resume = %+v, error = %v", result, err)
			}
			assertData(t, path, data)
			mu.Lock()
			defer mu.Unlock()
			sort.Strings(requests)
			if !reflect.DeepEqual(requests, []string{"bytes=0-0", "bytes=0-3", "bytes=8-11"}) {
				t.Fatalf("failed chunk was not replaced in full: %v", requests)
			}
		})
	}
}

func TestEmptyRangeResumeRefusesChangedMetadata(t *testing.T) {
	for _, mode := range []string{"etag", "length", "unknown length", "range"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Length", "0")
				switch mode {
				case "etag":
					w.Header().Set("ETag", `"changed"`)
				case "length":
					w.Header().Set("Content-Length", "1")
					fmt.Fprint(w, "x")
				case "unknown length":
					w.Header().Del("Content-Length")
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				case "range":
					w.Header().Set("Content-Range", "bytes */0")
				}
			}))
			defer srv.Close()
			path := rangeFixture(t, srv.URL, "", 4, nil)
			before, meta := rangeArtifacts(t, path)
			result, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Resume: true}, nil)
			if err == nil || result.Published {
				t.Fatalf("accepted empty checkpoint change: %+v, %v", result, err)
			}
			after, saved := rangeArtifacts(t, path)
			if !bytes.Equal(before, after) || !bytes.Equal(meta, saved) {
				t.Fatal("changed refused empty checkpoint")
			}
			assertAbsent(t, path)
		})
	}
}

func rangeFixture(t *testing.T, raw, data string, size int64, done []ranges.Span) string {
	t.Helper()
	path, f, _ := workerFile(t, raw, int64(len(data)), size)
	if err := os.WriteFile(path+storage.PartSuffix, bytes.Repeat([]byte("?"), len(data)), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, span := range done {
		w, err := f.ClaimRange(span)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data[span.Start:span.End])); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		w.Close()
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func rangeArtifacts(t *testing.T, path string) ([]byte, []byte) {
	t.Helper()
	data, err := os.ReadFile(path + storage.PartSuffix)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil {
		t.Fatal(err)
	}
	return data, meta
}
