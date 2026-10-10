package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/storage"
)

func TestProcessCrashResume(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), storage.CheckpointBytes/8)
	var offset atomic.Int64
	var resumed atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"process-version"`)

		if r.Header.Get("Range") == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data[:storage.CheckpointBytes+64*1024])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}

		start := offset.Load()
		if r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", start) || r.Header.Get("If-Range") != `"process-version"` {
			t.Error("process did not request the checkpoint suffix with its validator")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resumed.Store(true)
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data))-start, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start:])
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	cmd := command("download", srv.URL, "--output", path, "--quiet")
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false

	defer func() {
		if !finished {
			cmd.Process.Kill()
			<-done
		}
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ready := false

	for !ready {
		select {
		case <-ticker.C:
			m, err := storage.Inspect(path)
			ready = err == nil && m.Completed > 0
		case err := <-done:
			finished = true
			t.Fatalf("process stopped before crash injection: %v; %s", err, &errOut)
		case <-deadline.C:
			t.Fatal("process never published a checkpoint")
		}
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		finished = true
		if err == nil {
			t.Fatal("killed process unexpectedly succeeded")
		}
	case <-deadline.C:
		t.Fatal("process did not terminate")
	}

	m, err := storage.Inspect(path)
	if err != nil || m.Completed == 0 || m.Completed >= int64(len(data)) {
		t.Fatalf("invalid crash checkpoint: %+v, %v", m, err)
	}

	offset.Store(m.Completed)
	out, err := command("inspect", path).CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("Saved bytes:")) {
		t.Fatalf("inspect after crash = %q, %v", out, err)
	}

	out, err = command("download", srv.URL, "--output", path, "--quiet", "--resume").CombinedOutput()
	if err != nil || string(out) != path+"\n" || !resumed.Load() {
		t.Fatalf("resume after crash = %q, %v", out, err)
	}

	got, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatalf("resumed content mismatch: %v", err)
	}

	for _, suffix := range []string{storage.PartSuffix, storage.ManifestSuffix} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("left completed artifact %s: %v", suffix, err)
		}
	}
}
