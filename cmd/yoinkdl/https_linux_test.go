package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestProcessHTTPS(t *testing.T) {
	data := make([]byte, 2*int(ranges.MinChunkSize)+17)
	for i := range data {
		data[i] = byte(i*31 + i/4096)
	}

	want := sha256.Sum256(data)

	for _, test := range []struct {
		name    string
		workers int
		trusted bool
		calls   int32
	}{
		{name: "sequential", workers: 1, trusted: true, calls: 1},
		{name: "parallel", workers: 4, trusted: true, calls: 4},
		{name: "untrusted", workers: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("ETag", `"https-fixture"`)
				http.ServeContent(w, r, "fixture", time.Time{}, bytes.NewReader(data))
			}))
			srv.Config.ErrorLog = log.New(io.Discard, "", 0)
			srv.StartTLS()
			defer srv.Close()

			dir := t.TempDir()
			cert := filepath.Join(dir, "roots.pem")
			var roots []byte
			if test.trusted {
				roots = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
			}

			if err := os.WriteFile(cert, roots, 0o600); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(dir, "file")
			cmd := command("download", srv.URL, "--output", path, "--connections", strconv.Itoa(test.workers),
				"--retries", "0", "--timeout", "5s", "--quiet")
			cmd.Env = append(cmd.Env, "SSL_CERT_FILE="+cert, "SSL_CERT_DIR="+t.TempDir())
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()

			if calls.Load() != test.calls {
				t.Fatalf("HTTP requests = %d, want %d", calls.Load(), test.calls)
			}

			if test.trusted {
				if err != nil || string(out) != path+"\n" || stderr.Len() != 0 {
					t.Fatalf("HTTPS download = %q, error = %v, stderr = %s", out, err, &stderr)
				}

				got, err := os.ReadFile(path)
				if err != nil || sha256.Sum256(got) != want {
					t.Fatalf("HTTPS fixture mismatch: %v", err)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 1 || len(out) != 0 {
					t.Fatalf("untrusted HTTPS = %q, error = %v, stderr = %s", out, err, &stderr)
				}

				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("untrusted response published output: %v", err)
				}
			}

			for _, suffix := range []string{storage.PartSuffix, storage.ManifestSuffix} {
				if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unexpected HTTPS artifact %s: %v", suffix, err)
				}
			}
		})
	}
}
