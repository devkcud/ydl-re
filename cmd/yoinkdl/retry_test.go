package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"yoinkdl/internal/transfer"
)

func TestProcessRetryBudgets(t *testing.T) {
	for _, mode := range []string{"default", "one", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			failures := int32(transfer.DefaultRetries)
			if mode != "default" {
				failures = 1
			}
			var probes, chunks atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				probe := r.Header.Get("If-Range") == ""
				n := int32(0)
				if probe {
					n = probes.Add(1)
				} else {
					n = chunks.Add(1)
				}
				if n <= failures {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("ETag", `"v1"`)
				body := "contents"
				end := len(body) - 1
				if probe {
					body = body[:1]
					end = 0
				}
				if r.Header.Get("Range") != fmt.Sprintf("bytes=0-%d", end) {
					t.Error("retry changed the assigned bounds")
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/8", end))
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, body)
			}))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file")
			args := []string{"download", srv.URL, "-o", path, "--quiet"}
			if mode == "one" {
				args = append(args, "--retries", "1")
			} else if mode == "disabled" {
				args = append(args, "--retries", "0")
			}
			out, err := command(args...).CombinedOutput()
			if mode == "disabled" {
				if err == nil || !strings.Contains(string(out), "HTTP 503") || probes.Load() != 1 || chunks.Load() != 0 {
					t.Fatalf("disabled retries: out=%q error=%v probes=%d chunks=%d", out, err, probes.Load(), chunks.Load())
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("published a failed request")
				}
				return
			}
			if err != nil || string(out) != path+"\n" || probes.Load() != failures+1 || chunks.Load() != failures+1 {
				t.Fatalf("retry CLI: out=%q error=%v probes=%d chunks=%d", out, err, probes.Load(), chunks.Load())
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "contents" {
				t.Fatalf("retried output=%q, error=%v", data, err)
			}
		})
	}
}
