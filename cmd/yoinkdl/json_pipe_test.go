//go:build linux || darwin

package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"yoinkdl/internal/storage"
)

func TestProcessJSONBrokenPipe(t *testing.T) {
	for _, phase := range []string{"progress", "terminal", "closed_stderr"} {
		t.Run(phase, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"v1"`)
				body := "abcdefghij"
				if phase != "terminal" {
					if r.Header.Get("Range") != "bytes=3-" || r.Header.Get("If-Range") != `"v1"` {
						t.Error("broken-pipe fixture lost saved identity")
					}
					w.Header().Set("Content-Range", "bytes 3-9/10")
					w.WriteHeader(http.StatusPartialContent)
					body = body[3:]
				}
				fmt.Fprint(w, body)
			}))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file")
			raw := srv.URL + "?token=private-secret"
			args := []string{"download", raw, "-o", path, "--json", "-c", "1"}
			if phase != "terminal" {
				f, err := storage.Create(path, false)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				id := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
				if err := f.Begin(storage.Manifest{SourceID: id, EffectiveID: id, Length: 10, ETag: `"v1"`, Encoding: "identity"}); err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write([]byte("abc")); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				args = append(args, "--quiet")
			}
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			reader.Close()
			defer writer.Close()
			cmd := command(args...)
			if phase == "closed_stderr" {
				cmd.Env = append(cmd.Env, "YOINKDL_TEST_CLOSE_STDERR=1")
			}
			var errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = writer, &errOut
			done := startJSONProcess(t, cmd)
			writer.Close()
			waitJSONExit(t, done, 1)
			if phase != "closed_stderr" && !strings.Contains(errOut.String(), "JSON output failed") || strings.Contains(errOut.String(), "private-secret") {
				t.Fatalf("broken pipe diagnostic = %q", &errOut)
			}
			if phase != "terminal" {
				if got, err := os.ReadFile(path + storage.PartSuffix); err != nil || string(got) != "abc" {
					t.Fatalf("broken pipe lost saved bytes: %q, %v", got, err)
				}
				m, err := storage.Inspect(path)
				if err != nil || m.Completed != 3 || (phase != "closed_stderr" && !strings.Contains(errOut.String(), "Partial file preserved:")) {
					t.Fatalf("broken-pipe checkpoint = %+v, %v; %s", m, err, &errOut)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("broken progress output published an incomplete file")
				}
			} else {
				if got, err := os.ReadFile(path); err != nil || string(got) != "abcdefghij" {
					t.Fatalf("failed result output lost published file: %q, %v", got, err)
				}
				if !strings.Contains(errOut.String(), "Verified output published:") {
					t.Fatalf("hidden publication: %q", &errOut)
				}
			}
		})
	}
}

func TestProcessJSONCancelsFullStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	filled := fillJSONPipe(t, writer)
	path := filepath.Join(t.TempDir(), "file")
	cmd := command("download", srv.URL, "-o", path, "--json", "-c", "1")
	var errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = writer, &errOut
	done := startJSONProcess(t, cmd)
	writer.Close()
	waitJSONState(t, done, func() bool {
		_, err := os.Stat(path + storage.ManifestSuffix)
		return err == nil
	})
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	waitJSONState(t, done, func() bool {
		_, err := os.Stat(path + storage.PartSuffix)
		return os.IsNotExist(err)
	})
	if _, err := io.CopyN(io.Discard, reader, int64(filled)); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	waitJSONExit(t, done, 130)
	result, count := processJSONResult(t, data)
	if errOut.Len() != 0 || count != 1 || result.Status != "interrupted" || result.Published || result.Bytes != 0 || result.Partial != "" {
		t.Fatalf("full-pipe interruption: result=%+v, stderr=%q", result, &errOut)
	}
}

func fillJSONPipe(t *testing.T, file *os.File) int {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	filled := 0
	if err := raw.Control(func(fd uintptr) {
		buf := make([]byte, 65536)
		for {
			n, err := syscall.Write(int(fd), buf)
			filled += max(n, 0)
			if errors.Is(err, syscall.EAGAIN) {
				return
			}
			if err != nil && !errors.Is(err, syscall.EINTR) {
				t.Error(err)
				return
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	return filled
}
