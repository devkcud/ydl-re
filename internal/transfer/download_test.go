package transfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/storage"
)

func TestDownload(t *testing.T) {
	for _, mode := range []string{"known", "unknown", "empty", "encoded"} {
		t.Run(mode, func(t *testing.T) {
			data := bytes.Repeat([]byte("yoink\x00\xff"), 16384)

			if mode == "empty" {
				data = nil
			}

			if mode == "encoded" {
				var buf bytes.Buffer
				writer := gzip.NewWriter(&buf)

				if _, err := writer.Write(data); err != nil {
					t.Fatal(err)
				}

				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}

				data = buf.Bytes()
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept-Encoding") != "identity" || r.Header.Get("Range") != "bytes=0-0" {
					t.Error("unexpected representation or range request")
				}

				if mode == "encoded" {
					w.Header().Set("Content-Encoding", "gzip")
				}

				if mode == "unknown" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				} else {
					w.Header().Set("Content-Length", strconv.Itoa(len(data)))
				}

				w.Write(data)
			}))
			defer srv.Close()

			path := filepath.Join(t.TempDir(), "file")
			var snapshots []Snapshot
			result, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, func(s Snapshot) {
				snapshots = append(snapshots, s)
			})

			if err != nil {
				t.Fatal(err)
			}

			got, err := os.ReadFile(path)
			if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
				t.Fatalf("output mismatch: %v", err)
			}

			if result.Bytes != int64(len(data)) || !result.Published || result.Partial != "" {
				t.Fatalf("incorrect result: %+v", result)
			}

			assertAbsent(t, path+storage.PartSuffix)
			last := snapshots[len(snapshots)-1]

			if last.Bytes != result.Bytes || (mode == "unknown" && last.Total != -1) {
				t.Fatalf("incorrect progress: %+v", last)
			}
		})
	}
}

func TestHTTPFailures(t *testing.T) {
	for _, code := range []int{204, 206, 401, 404, 500} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			path := filepath.Join(t.TempDir(), "file")
			result, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, nil)
			if err == nil || result.Published {
				t.Fatalf("HTTP %d reported success", code)
			}

			assertAbsent(t, path)
			assertAbsent(t, path+storage.PartSuffix)
		})
	}
}

func TestShortResponsePreservesPartial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, "partial")
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, nil)
	if err == nil || result.Published || result.Partial != path+storage.PartSuffix {
		t.Fatalf("short response: result = %+v, error = %v", result, err)
	}

	assertAbsent(t, path)
	assertData(t, result.Partial, "partial")
}

func TestCancelPreservesPartial(t *testing.T) {
	srv := stalledServer()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := filepath.Join(t.TempDir(), "file")

	result, err := Download(ctx, Request{URL: srv.URL, Output: path}, func(s Snapshot) {
		if s.Bytes > 0 {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) || result.Published {
		t.Fatalf("cancellation: result = %+v, error = %v", result, err)
	}

	assertAbsent(t, path)
	assertData(t, result.Partial, "partial")
}

func TestIdleTimeout(t *testing.T) {
	srv := stalledServer()
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{
		URL: srv.URL, Output: path, Timeout: 200 * time.Millisecond,
	}, nil)

	if !errors.Is(err, context.DeadlineExceeded) || result.Published {
		t.Fatalf("idle timeout: result = %+v, error = %v", result, err)
	}

	assertAbsent(t, path)
	assertData(t, result.Partial, "partial")
}

func TestHeaderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	_, err := Download(context.Background(), Request{
		URL: srv.URL, Output: path, Timeout: 100 * time.Millisecond,
	}, nil)

	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected header timeout, got %v", err)
	}

	assertAbsent(t, path)
	assertAbsent(t, path+storage.PartSuffix)
}

func TestActiveTransferCanOutlastTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "6")
		ticker := time.NewTicker(60 * time.Millisecond)
		defer ticker.Stop()

		for range 6 {
			fmt.Fprint(w, "x")
			w.(http.Flusher).Flush()

			select {
			case <-ticker.C:
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	_, err := Download(context.Background(), Request{
		URL: srv.URL, Output: path, Timeout: 200 * time.Millisecond,
	}, nil)

	if err != nil {
		t.Fatal(err)
	}

	assertData(t, path, "xxxxxx")
}

func TestExistingDestinationSkipsNetwork(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, nil); err == nil {
		t.Fatal("accepted existing destination")
	}

	assertData(t, path, "original")

	if calls.Load() != 0 {
		t.Fatal("sent request before rejecting destination")
	}
}

func TestPublicationFailurePreservesPartial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "download")
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, func(s Snapshot) {
		if s.Bytes > 0 {
			if err := os.WriteFile(path, []byte("other file"), 0o600); err != nil {
				t.Error(err)
			}
		}
	})

	if err == nil || result.Published {
		t.Fatal("published over another file")
	}

	assertData(t, path, "other file")
	assertData(t, result.Partial, "download")
}

func TestRedirectAndSecretRedaction(t *testing.T) {
	for _, mode := range []string{"success", "loop", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "success" && r.URL.Path == "/file" {
					fmt.Fprint(w, "redirected")
					return
				}

				location := "/file"
				if mode == "malformed" {
					location = "http://invalid/%zz?token=secret-value"
				}

				w.Header().Set("Location", location)
				w.WriteHeader(http.StatusFound)
			}))
			defer srv.Close()

			path := filepath.Join(t.TempDir(), "file")
			_, err := Download(context.Background(), Request{URL: srv.URL + "?token=secret-value", Output: path}, nil)

			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}

				assertData(t, path, "redirected")
				return
			}

			if err == nil || strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("missing or unsafe error: %v", err)
			}

			assertAbsent(t, path)
			assertAbsent(t, path+storage.PartSuffix)
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	for _, raw := range []string{"", "ftp://example.org/file", "http://", "https://user:secret@example.org/file", "http://example.org/%zz?token=secret"} {
		path := filepath.Join(t.TempDir(), "file")
		_, err := Download(context.Background(), Request{URL: raw, Output: path}, nil)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("missing or unsafe input error: %v", err)
		}

		assertAbsent(t, path+storage.PartSuffix)
	}
}

func stalledServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, "partial")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
}

func assertData(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)

	if err != nil || string(got) != want {
		t.Fatalf("%s: data = %q, error = %v; want %q", path, got, err, want)
	}
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s should be absent: %v", path, err)
	}
}
