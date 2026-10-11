package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestProcessAggregateRateLimit(t *testing.T) {
	const rate = 8 * 1024 * 1024
	data := bytes.Repeat([]byte("abcdefgh"), 9*1024*1024/8)
	var chunks atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		if n != 2 || err != nil || start < 0 || end < start || end >= int64(len(data)) {
			t.Error("invalid limited range request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("If-Range") != "" {
			chunks.Add(1)
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	cmd := command("download", srv.URL, "-o", path, "--connections", "4", "--limit", "8MiB/s", "--quiet")
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	line, readErr := bufio.NewReader(stdout).ReadString('\n')
	elapsed := time.Since(start)
	err = cmd.Wait()
	if err != nil || readErr != nil || line != path+"\n" || errOut.Len() != 0 || chunks.Load() != 3 {
		t.Fatalf("limited CLI: out=%q, errors=%v/%v, stderr=%q, chunks=%d", line, err, readErr, &errOut, chunks.Load())
	}
	want := time.Duration(len(data)+1-65536) * time.Second / rate
	if elapsed+5*time.Millisecond < want {
		t.Fatalf("workers exceeded aggregate cap: elapsed=%s, minimum=%s", elapsed, want)
	}
	got, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatalf("limited process fixture mismatch: %v", err)
	}
}
