package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"yoinkdl/internal/ranges"
)

func TestProcessParallelDownload(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefgh"), int(ranges.MinChunkSize/8)+1)
	var chunks atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		if n != 2 || err != nil || start < 0 || end < start || end >= int64(len(data)) {
			t.Error("invalid process range request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("If-Range") != "" {
			if r.Header.Get("If-Range") != `"v1"` {
				t.Error("process lost the selected validator")
			}
			chunks.Add(1)
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	out, err := command("download", srv.URL, "--output", path, "--connections", "2", "--quiet").CombinedOutput()
	if err != nil || string(out) != path+"\n" || chunks.Load() != 2 {
		t.Fatalf("parallel CLI = %q, error = %v, chunks = %d", out, err, chunks.Load())
	}
	got, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatalf("parallel process fixture mismatch: %v", err)
	}
}
