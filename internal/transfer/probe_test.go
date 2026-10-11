package transfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
)

func TestProbeRangeCapability(t *testing.T) {
	for _, mode := range []string{"fixed", "chunked", "one byte", "maximum length", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			length := int64(10)
			if mode == "one byte" {
				length = 1
			} else if mode == "maximum length" {
				length = math.MaxInt64
			}

			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assertProbeRequest(t, r, true)
				if mode == "redirect" && r.URL.Path == "/source" {
					http.Redirect(w, r, "/file", http.StatusFound)
					return
				}

				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", length))
				w.Header().Set("ETag", `"v1"`)
				if mode != "chunked" {
					w.Header().Set("Content-Length", "1")
				}

				w.WriteHeader(http.StatusPartialContent)
				if mode == "chunked" {
					w.(http.Flusher).Flush()
				}

				fmt.Fprint(w, "a")
			}))
			defer srv.Close()

			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL + "/source")
			result, err := probe(context.Background(), client, source, retryPolicy{})
			if err != nil || result.stream != nil {
				t.Fatalf("probe = %+v, error = %v", result, err)
			}

			m := result.identity.metadata()
			effective := source
			wantCalls := int32(1)
			if mode == "redirect" {
				effective, _ = parseURL(srv.URL + "/file")
				wantCalls = 2
			}

			if m.Length != length || m.ETag != `"v1"` || m.SourceID != fingerprint(source) ||
				m.EffectiveID != fingerprint(effective) || calls.Load() != wantCalls {
				t.Fatalf("selected metadata = %+v, requests = %d", m, calls.Load())
			}
		})
	}
}

func TestProbeReusesFullResponse(t *testing.T) {
	for _, mode := range []string{"known", "unknown", "encoded", "empty"} {
		t.Run(mode, func(t *testing.T) {
			data := []byte("complete response")
			if mode == "empty" {
				data = nil
			} else if mode == "encoded" {
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

			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assertProbeRequest(t, r, true)
				w.Header().Set("Accept-Ranges", "bytes")
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

			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			result, err := probe(context.Background(), client, source, retryPolicy{})
			if err != nil || result.stream == nil || result.identity != (representation{}) {
				t.Fatalf("probe = %+v, error = %v", result, err)
			}
			defer result.stream.Body.Close()

			got, err := io.ReadAll(result.stream.Body)
			if err != nil || !bytes.Equal(got, data) || calls.Load() != 1 {
				t.Fatalf("body = %q, error = %v, requests = %d", got, err, calls.Load())
			}

			m := responseMetadata(result.stream, source)
			if mode == "unknown" && m.Length != -1 {
				t.Fatal("invented a total for a full unknown-length response")
			}

			if mode == "encoded" && (m.Encoding != "encoded" || result.stream.Uncompressed) {
				t.Fatal("fallback changed the served representation")
			}
		})
	}
}

func TestProbeFallsBackWithoutReusingBytes(t *testing.T) {
	for _, mode := range []string{"missing etag", "weak etag", "duplicate etag", "encoded", "duplicate encoding", "unknown total", "416", "empty 416"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) > 1 {
					assertProbeRequest(t, r, false)
					w.Header().Set("ETag", `"different-version"`)
					if mode == "empty 416" {
						w.Header().Set("Content-Length", "0")
					} else {
						fmt.Fprint(w, "new version")
					}
					return
				}

				assertProbeRequest(t, r, true)
				w.Header().Set("Content-Range", "bytes 0-0/10")
				w.Header().Set("ETag", `"old-version"`)
				w.Header().Set("Content-Length", "1")

				switch mode {
				case "missing etag":
					w.Header().Del("ETag")
				case "weak etag":
					w.Header().Set("ETag", `W/"old-version"`)
				case "duplicate etag":
					w.Header().Add("ETag", `"old-version"`)
				case "encoded":
					w.Header().Set("Content-Encoding", "gzip")
				case "duplicate encoding":
					w.Header()["Content-Encoding"] = []string{"identity", "identity"}
				case "unknown total":
					w.Header().Set("Content-Range", "bytes 0-0/*")
				case "416", "empty 416":
					w.Header().Set("Content-Range", "bytes */0")
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					fmt.Fprint(w, "x")
					return
				}

				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, "x")
			}))
			defer srv.Close()
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			result, err := probe(context.Background(), client, source, retryPolicy{})
			if err != nil || result.stream == nil || result.identity != (representation{}) {
				t.Fatalf("fallback = %+v, error = %v", result, err)
			}
			defer result.stream.Body.Close()

			want := "new version"
			if mode == "empty 416" {
				want = ""
			}

			got, err := io.ReadAll(result.stream.Body)
			m := responseMetadata(result.stream, source)
			if err != nil || string(got) != want || m.ETag != `"different-version"` || m.Length != int64(len(want)) || calls.Load() != 2 {
				t.Fatalf("fallback body = %q, metadata = %+v, error = %v, requests = %d", got, m, err, calls.Load())
			}
		})
	}
}

func TestProbeRejectsInvalidResponses(t *testing.T) {
	for _, mode := range []string{
		"404", "500", "204", "missing range", "duplicate range", "start", "end", "zero total", "overflow",
		"wrong length", "short body", "empty chunked", "long chunked", "broken body", "full response range",
	} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Range", "bytes 0-0/10")
				w.Header().Set("ETag", `"v1"`)
				code := http.StatusPartialContent

				switch mode {
				case "404", "500", "204":
					code, _ = strconv.Atoi(mode)
				case "missing range":
					w.Header().Del("Content-Range")
				case "duplicate range":
					w.Header().Add("Content-Range", "bytes 0-0/10")
				case "start":
					w.Header().Set("Content-Range", "bytes 1-1/10")
				case "end":
					w.Header().Set("Content-Range", "bytes 0-1/10")
				case "zero total":
					w.Header().Set("Content-Range", "bytes 0-0/0")
				case "overflow":
					w.Header().Set("Content-Range", "bytes 0-0/9223372036854775808")
				case "wrong length":
					w.Header().Set("Content-Length", "2")
				case "short body":
					w.Header().Set("Content-Length", "1")
				case "full response range":
					code = http.StatusOK
				}

				w.WriteHeader(code)
				w.(http.Flusher).Flush()
				if mode == "short body" || mode == "empty chunked" {
					return
				}

				fmt.Fprint(w, "a")
				if mode == "long chunked" {
					fmt.Fprint(w, "b")
				} else if mode == "broken body" {
					w.(http.Flusher).Flush()
					panic(http.ErrAbortHandler)
				}
			}))
			defer srv.Close()
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL + "?token=private-secret")
			result, err := probe(context.Background(), client, source, retryPolicy{})
			if err == nil || result.stream != nil || result.identity != (representation{}) || calls.Load() != 1 || strings.Contains(err.Error(), "private-secret") {
				t.Fatalf("invalid probe = %+v, error = %v, requests = %d", result, err, calls.Load())
			}
		})
	}
}

func TestProbeFallbackIsBounded(t *testing.T) {
	for _, code := range []int{http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable, http.StatusNotFound, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}

				assertProbeRequest(t, r, false)
				w.WriteHeader(code)
			}))
			defer srv.Close()
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			if _, err := probe(context.Background(), client, source, retryPolicy{}); err == nil || calls.Load() != 2 {
				t.Fatalf("fallback error = %v, requests = %d", err, calls.Load())
			}
		})
	}
}

func TestProbePinsSubsequentResponses(t *testing.T) {
	for _, mode := range []string{"same", "etag", "redirect", "total", "ignored"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("ETag", `"v1"`)
					w.Header().Set("Content-Range", "bytes 0-0/10")
					w.WriteHeader(http.StatusPartialContent)
					fmt.Fprint(w, "a")
					return
				}

				if r.Header.Get("If-Range") != `"v1"` || r.Header.Get("Range") != "bytes=3-5" {
					t.Error("subsequent request lost the selected identity or range")
				}

				if mode == "redirect" && r.URL.Path != "/changed" {
					http.Redirect(w, r, "/changed", http.StatusFound)
					return
				}

				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Range", "bytes 3-5/10")
				if mode == "etag" {
					w.Header().Set("ETag", `"v2"`)
				} else if mode == "total" {
					w.Header().Set("Content-Range", "bytes 3-5/11")
				}

				if mode != "ignored" {
					w.WriteHeader(http.StatusPartialContent)
				}

				fmt.Fprint(w, "def")
			}))
			defer srv.Close()
			client := newClient(time.Second, nil)
			defer client.CloseIdleConnections()
			source, _ := parseURL(srv.URL)
			result, err := probe(context.Background(), client, source, retryPolicy{})
			if err != nil || result.stream != nil {
				t.Fatalf("probe = %+v, error = %v", result, err)
			}

			span := ranges.Span{Start: 3, End: 6}
			req, err := result.identity.request(context.Background(), source, span)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if err := result.identity.validate(resp, span); (err == nil) != (mode == "same") {
				t.Fatalf("subsequent range validation = %v", err)
			}
		})
	}
}

func assertProbeRequest(t *testing.T, req *http.Request, ranged bool) {
	t.Helper()
	want := ""
	if ranged {
		want = "bytes=0-0"
	}

	if req.Method != http.MethodGet || req.Header.Get("Range") != want ||
		req.Header.Get("Accept-Encoding") != "identity" || req.Header.Get("User-Agent") != "yoinkdl" ||
		req.Header.Get("If-Range") != "" || req.Header.Get("If-Match") != "" {
		t.Error("invalid fresh capability or fallback request")
	}
}
