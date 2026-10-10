package transfer

import (
	"bytes"
	"context"
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

func TestResume(t *testing.T) {
	for _, mode := range []string{"automatic", "explicit", "chunked", "zero-prefix"} {
		t.Run(mode, func(t *testing.T) {
			const data = "abcdefghij"
			var calls atomic.Int32
			offset := 3
			if mode == "zero-prefix" {
				offset = 0
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"version-1"`)

				if calls.Add(1) == 1 {
					w.Header().Set("Content-Length", "10")
					if mode == "zero-prefix" {
						fmt.Fprint(w, data)
						return
					}

					fmt.Fprint(w, data[:offset])
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}

				if offset == 0 {
					if r.Header.Get("If-Match") != `"version-1"` || r.Header.Get("Range") != "" {
						t.Error("zero-prefix recovery did not validate a fresh full request")
					}

					w.Header().Set("Content-Length", "10")
					fmt.Fprint(w, data)
					return
				}

				if r.Header.Get("Range") != "bytes=3-" || r.Header.Get("If-Range") != `"version-1"` {
					t.Error("resume did not request the missing suffix conditionally")
				}

				w.Header().Set("Content-Range", "bytes 3-9/10")
				if mode != "chunked" {
					w.Header().Set("Content-Length", "7")
				}

				w.WriteHeader(http.StatusPartialContent)
				if mode == "chunked" {
					w.(http.Flusher).Flush()
				}

				fmt.Fprint(w, data[offset:])
			}))
			defer srv.Close()

			url := srv.URL + "/file?token=private-query-token"
			path := interruptDownload(t, url)
			before, err := os.ReadFile(path + storage.ManifestSuffix)
			if err != nil || strings.Contains(string(before), "private-query-token") || strings.Contains(string(before), srv.URL) {
				t.Fatalf("unsafe metadata: %v", err)
			}

			var snapshots []Snapshot
			result, err := Download(context.Background(), Request{
				URL: url, Output: path, Resume: mode == "explicit",
			}, func(s Snapshot) { snapshots = append(snapshots, s) })

			if err != nil || !result.Published || result.Resumed != int64(offset) || result.Bytes != 10 {
				t.Fatalf("result = %+v, error = %v", result, err)
			}

			assertData(t, path, data)
			assertAbsent(t, path+storage.PartSuffix)
			assertAbsent(t, path+storage.ManifestSuffix)

			if snapshots[0].Bytes != int64(offset) || snapshots[0].Resumed != int64(offset) || snapshots[len(snapshots)-1].Bytes != 10 {
				t.Fatalf("incorrect resumed progress: %+v", snapshots)
			}
		})
	}
}

func TestResumeRefusesIncompatibleResponses(t *testing.T) {
	modes := []string{
		"etag", "ignored", "start", "end", "total", "unknown-total", "junk", "overflow",
		"missing-etag", "duplicate-etag", "duplicate-range", "encoding", "payload-length", "416", "redirect",
	}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				w.Header().Set("ETag", `"version-1"`)

				if call == 1 {
					w.Header().Set("Content-Length", "10")
					fmt.Fprint(w, "abc")
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}

				if mode == "416" {
					if call == 2 {
						w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
						return
					}

					if r.Header.Get("Range") != "" {
						t.Error("re-probe retained a rejected range")
					}

					fmt.Fprint(w, "abcdefghij")
					return
				}

				if mode == "redirect" && r.URL.Path != "/moved" {
					w.Header().Set("Location", "/moved")
					w.WriteHeader(http.StatusFound)
					return
				}

				status := http.StatusPartialContent
				w.Header().Set("Content-Range", "bytes 3-9/10")
				w.Header().Set("Content-Length", "7")

				switch mode {
				case "etag":
					w.Header().Set("ETag", `"version-2"`)
				case "ignored":
					status = http.StatusOK
				case "start":
					w.Header().Set("Content-Range", "bytes 2-9/10")
				case "end":
					w.Header().Set("Content-Range", "bytes 3-8/10")
				case "total":
					w.Header().Set("Content-Range", "bytes 3-9/11")
				case "unknown-total":
					w.Header().Set("Content-Range", "bytes 3-9/*")
				case "junk":
					w.Header().Set("Content-Range", "bytes 3-9/10 trailing")
				case "overflow":
					w.Header().Set("Content-Range", "bytes 3-9/99999999999999999999999")
				case "missing-etag":
					w.Header().Del("ETag")
				case "duplicate-etag":
					w.Header().Add("ETag", `"version-2"`)
				case "duplicate-range":
					w.Header().Add("Content-Range", "bytes 3-9/10")
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				case "payload-length":
					w.Header().Set("Content-Length", "6")
				}

				w.WriteHeader(status)
				fmt.Fprint(w, "defghi")
			}))
			defer srv.Close()

			url := srv.URL + "/file"
			path := interruptDownload(t, url)
			if err := os.WriteFile(path+storage.PartSuffix, []byte("abcTAIL"), 0o600); err != nil {
				t.Fatal(err)
			}

			before, err := os.ReadFile(path + storage.ManifestSuffix)
			if err != nil {
				t.Fatal(err)
			}

			result, err := Download(context.Background(), Request{URL: url, Output: path, Resume: true}, nil)
			if err == nil || result.Published || result.Resumed != 0 {
				t.Fatalf("accepted incompatible response: %+v, %v", result, err)
			}

			assertAbsent(t, path)
			assertData(t, path+storage.PartSuffix, "abcTAIL")
			after, err := os.ReadFile(path + storage.ManifestSuffix)

			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("changed metadata before validating the resume")
			}

			if mode == "416" && calls.Load() != 3 {
				t.Fatal("did not re-probe after 416")
			}
		})
	}
}

func TestUnsafeRepresentationsDoNotResume(t *testing.T) {
	for _, mode := range []string{"missing-etag", "weak-etag", "unknown-length", "encoded"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("ETag", `"version-1"`)

				if mode != "unknown-length" {
					w.Header().Set("Content-Length", "10")
				}

				switch mode {
				case "missing-etag":
					w.Header().Del("ETag")
				case "weak-etag":
					w.Header().Set("ETag", `W/"version-1"`)
				case "encoded":
					w.Header().Set("Content-Encoding", "gzip")
				}

				fmt.Fprint(w, "abc")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer srv.Close()

			path := interruptDownload(t, srv.URL)
			_, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, nil)

			if err == nil || calls.Load() != 1 {
				t.Fatalf("attempted unsafe reuse: %v, calls = %d", err, calls.Load())
			}

			assertData(t, path+storage.PartSuffix, "abc")
			assertAbsent(t, path)
		})
	}
}

func TestChangedSourceRefusesBeforeNetwork(t *testing.T) {
	srv := resumableStall()
	defer srv.Close()
	path := interruptDownload(t, srv.URL+"?token=old-token")

	_, err := Download(context.Background(), Request{URL: srv.URL + "?token=new-token", Output: path}, nil)
	if err == nil || strings.Contains(err.Error(), "old-token") || strings.Contains(err.Error(), "new-token") {
		t.Fatalf("missing or unsafe source error: %v", err)
	}

	assertData(t, path+storage.PartSuffix, "abc")
}

func TestReadyCheckpointPublishesWithoutNetwork(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	u, err := parseURL(url)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	err = f.Begin(storage.Manifest{
		SourceID: fingerprint(u), EffectiveID: fingerprint(u), Length: 6, Encoding: "identity",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("collision"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := f.Finalize(); err == nil {
		t.Fatal("expected publication conflict")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	result, err := Download(context.Background(), Request{URL: url, Output: path, Resume: true}, nil)
	if err != nil || !result.Published || result.Resumed != 6 {
		t.Fatalf("ready recovery = %+v, error = %v", result, err)
	}

	assertData(t, path, "abcdef")
	assertAbsent(t, path+storage.ManifestSuffix)
}

func TestResumeShortBodyRetainsNewSafePrefix(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version-1"`)

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "10")
			fmt.Fprint(w, "abc")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}

		w.Header().Set("Content-Range", "bytes 3-9/10")
		w.Header().Set("Content-Length", strconv.Itoa(7))
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, "de")
	}))
	defer srv.Close()

	path := interruptDownload(t, srv.URL)
	_, err := Download(context.Background(), Request{URL: srv.URL, Output: path}, nil)
	if err == nil {
		t.Fatal("accepted incomplete resumed body")
	}

	m, err := storage.Inspect(path)
	if err != nil || m.Completed != 5 {
		t.Fatalf("checkpoint = %+v, error = %v", m, err)
	}

	assertData(t, path+storage.PartSuffix, "abcde")
	assertAbsent(t, path)
}

func interruptDownload(t *testing.T, url string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Download(ctx, Request{URL: url, Output: path}, func(s Snapshot) {
		if s.Bytes > 0 {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected interruption, got %v", err)
	}

	return path
}

func resumableStall() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version-1"`)
		w.Header().Set("Content-Length", "10")
		fmt.Fprint(w, "abc")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
}
