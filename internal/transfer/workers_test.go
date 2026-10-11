package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestParallelDownload(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		t.Run(fmt.Sprint(chunked), func(t *testing.T) {
			data := make([]byte, 3*ranges.MinChunkSize+123)
			for i := range data {
				data[i] = byte(i*31 + i/4096)
			}

			count := 2
			requested := count
			if chunked {
				count = DefaultConnections
				requested = 0
			}

			var started, active, peak atomic.Int32
			var mu sync.Mutex
			seen := make(map[int64]int)
			ready := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start, end := requestedRange(t, r, int64(len(data)))
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				if r.Header.Get("If-Range") == "" {
					if start != 0 || end != 0 {
						t.Error("unexpected capability request")
					}
					w.WriteHeader(http.StatusPartialContent)
					w.Write(data[:1])
					return
				}

				if r.Header.Get("If-Range") != `"v1"` {
					t.Error("missing immutable validator")
				}
				mu.Lock()
				seen[start]++
				mu.Unlock()
				n := active.Add(1)
				defer active.Add(-1)
				for old := peak.Load(); n > old; old = peak.Load() {
					if peak.CompareAndSwap(old, n) {
						break
					}
				}

				if started.Add(1) == int32(count) {
					close(ready)
				}
				select {
				case <-ready:
				case <-r.Context().Done():
					return
				}

				if !chunked {
					w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
				}
				w.WriteHeader(http.StatusPartialContent)
				if chunked {
					w.(http.Flusher).Flush()
				}
				w.Write(data[start : end+1])
			}))
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "file")
			var last Snapshot
			var callback atomic.Bool
			result, err := Download(ctx, Request{URL: srv.URL, Output: path, Connections: requested}, func(s Snapshot) {
				if !callback.CompareAndSwap(false, true) {
					t.Error("concurrent progress callbacks")
				}
				if s.Workers < 0 || s.Workers > count || s.Bytes < last.Bytes || s.Bytes > int64(len(data)) {
					t.Errorf("invalid progress: %+v after %+v", s, last)
				}
				last = s
				callback.Store(false)
			})
			if err != nil || !result.Published || result.Bytes != int64(len(data)) || result.Resumed != 0 {
				t.Fatalf("parallel download = %+v, error = %v", result, err)
			}

			got, err := os.ReadFile(path)
			if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
				t.Fatalf("parallel fixture mismatch: %v", err)
			}
			if peak.Load() != int32(count) || last.Workers != 0 || last.Bytes != int64(len(data)) {
				t.Fatalf("peak = %d, final progress = %+v", peak.Load(), last)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(seen) != 4 {
				t.Fatalf("requested %d chunks", len(seen))
			}
			for _, n := range seen {
				if n != 1 {
					t.Fatal("a chunk was requested more than once")
				}
			}
			assertAbsent(t, path+storage.PartSuffix)
			assertAbsent(t, path+storage.ManifestSuffix)
		})
	}
}

func TestWorkerFailuresPreserveCompletedChunks(t *testing.T) {
	for _, mode := range []string{"etag", "bounds", "ignored", "416", "short", "oversized", "broken", "cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") == "" {
					if mode != "416" {
						t.Error("unexpected destructive fallback")
					}
					fmt.Fprint(w, "new whole body")
					return
				}
				calls.Add(1)
				start, end := requestedRange(t, r, 9)
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/9", start, end))
				if start == 0 {
					w.WriteHeader(http.StatusPartialContent)
					fmt.Fprint(w, "abc")
					return
				}
				if start != 3 {
					t.Error("started new work after the failed chunk")
				}
				switch mode {
				case "etag":
					w.Header().Set("ETag", `"changed"`)
				case "bounds":
					w.Header().Set("Content-Range", "bytes 2-4/9")
				case "ignored":
					fmt.Fprint(w, "whole body")
					return
				case "416":
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				w.WriteHeader(http.StatusPartialContent)
				w.(http.Flusher).Flush()
				if mode == "oversized" {
					fmt.Fprint(w, "defX")
				} else if mode == "short" || mode == "cancel" || mode == "timeout" {
					fmt.Fprint(w, "d")
					w.(http.Flusher).Flush()
					if mode != "short" {
						<-r.Context().Done()
					}
				} else {
					fmt.Fprint(w, "def")
					if mode == "broken" {
						w.(http.Flusher).Flush()
						panic(http.ErrAbortHandler)
					}
				}
			}))
			defer srv.Close()
			path, f, id := workerFile(t, srv.URL, 9, 3)
			client := newClient(100*time.Millisecond, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := receiveRanges(ctx, client, source, id, f, 1, retryPolicy{}, func(s Snapshot) {
				if mode == "cancel" && s.Bytes > 3 {
					cancel()
				}
			})
			if err == nil || (mode == "cancel" && !errors.Is(err, context.Canceled)) ||
				(mode == "timeout" && !errors.Is(err, context.DeadlineExceeded)) {
				t.Fatalf("worker failure = %v", err)
			}
			if err := f.Close(); err != nil {
				t.Fatalf("worker retained a writer claim: %v", err)
			}
			m, err := storage.Inspect(path)
			if err != nil || m.Completed != 3 || len(m.Ranges) != 1 || calls.Load() != 2 {
				t.Fatalf("unsafe checkpoint = %+v, error = %v, requests = %d", m, err, calls.Load())
			}
			assertAbsent(t, path)
			got, err := os.ReadFile(path + storage.PartSuffix)
			if err != nil || !bytes.HasPrefix(got, []byte("abc")) || len(got) > 6 {
				t.Fatalf("neighboring chunk was overwritten: %q, %v", got, err)
			}
		})
	}
}

func TestParallelFailureCancelsOtherWorkers(t *testing.T) {
	var started atomic.Int32
	ready := make(chan struct{})
	stopped := make(chan struct{}, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { stopped <- struct{}{} }()
		start, end := requestedRange(t, r, 9)
		if started.Add(1) == 2 {
			close(ready)
		}
		select {
		case <-ready:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/9", start, end))
		if start == 0 {
			w.Header().Set("ETag", `"changed"`)
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, "bad")
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	_, f, id := workerFile(t, srv.URL, 9, 3)
	client := newClient(time.Second, nil)
	defer client.CloseIdleConnections()
	source, _ := parseURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := receiveRanges(ctx, client, source, id, f, 2, retryPolicy{}, nil); err == nil {
		t.Fatal("accepted a changed representation")
	}
	for range 2 {
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("a failed transfer left an HTTP worker active")
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestParallelCancellationBeforePublication(t *testing.T) {
	const data = "abcdefghij"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := requestedRange(t, r, int64(len(data)))
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, data[start:end+1])
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(ctx, Request{URL: srv.URL, Output: path}, func(s Snapshot) {
		if s.Bytes == int64(len(data)) && s.Workers == 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || result.Published {
		t.Fatalf("canceled download = %+v, error = %v", result, err)
	}
	assertAbsent(t, path)
	assertData(t, path+storage.PartSuffix, data)
	m, err := storage.Inspect(path)
	if err != nil || m.Completed != int64(len(data)) || m.State != storage.StatePartial {
		t.Fatalf("canceled checkpoint = %+v, error = %v", m, err)
	}
	f, err := storage.Open(context.Background(), path, false, true)
	if err != nil {
		t.Fatalf("canceled download retained ownership: %v", err)
	}
	f.Close()
}

func TestInvalidWorkerConfigurationSkipsNetwork(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()
	_, f, id := workerFile(t, srv.URL, 9, 3)
	client := newClient(time.Second, nil)
	defer client.CloseIdleConnections()
	source, _ := parseURL(srv.URL)
	for _, count := range []int{0, -1, MaxConnections + 1} {
		if _, err := receiveRanges(context.Background(), client, source, id, f, count, retryPolicy{}, nil); err == nil {
			t.Errorf("accepted invalid worker count %d", count)
		}
	}
	changed := id
	changed.etag = `"different"`
	if _, err := receiveRanges(context.Background(), client, source, changed, f, 2, retryPolicy{}, nil); err == nil {
		t.Fatal("accepted a different storage identity")
	}
	for _, count := range []int{-1, MaxConnections + 1} {
		path := filepath.Join(t.TempDir(), "file")
		if _, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Connections: count}, nil); err == nil {
			t.Fatal("download accepted an invalid connection limit")
		}
		assertAbsent(t, path+storage.PartSuffix)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid worker settings started HTTP work")
	}
}

func requestedRange(t *testing.T, r *http.Request, length int64) (int64, int64) {
	t.Helper()
	var start, end int64
	n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
	if n != 2 || err != nil || start < 0 || end < start || end >= length || r.Header.Get("Accept-Encoding") != "identity" {
		t.Errorf("invalid range request: %q", r.Header.Get("Range"))
		return 0, 0
	}
	return start, end
}

func workerFile(t *testing.T, raw string, length, size int64) (string, *storage.File, representation) {
	t.Helper()
	source, _ := parseURL(raw)
	id := representation{sourceID: fingerprint(source), effectiveID: fingerprint(source), length: length, etag: `"v1"`}
	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	m := id.metadata()
	m.ChunkSize = size
	if err := f.BeginRanges(m); err != nil {
		t.Fatal(err)
	}
	return path, f, id
}
