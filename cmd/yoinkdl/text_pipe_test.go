//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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

func TestProcessTextBrokenPipes(t *testing.T) {
	for _, mode := range []string{"help", "diagnostic", "progress", "result", "quiet", "json diagnostic"} {
		t.Run(mode, func(t *testing.T) {
			srv := textServer(t, nil)
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file")
			args := []string{"download", srv.URL, "-o", path, "-c", "1"}
			code := 1
			switch mode {
			case "help":
				args = []string{"help"}
			case "diagnostic":
				args = []string{"unknown"}
				code = 2
			case "progress":
				textCheckpoint(t, path, srv.URL)
			case "result", "quiet":
				args = append(args, "--quiet")
				if mode == "quiet" {
					code = 0
				}
			case "json diagnostic":
				args = []string{"download", "not-a-url", "--json"}
			}
			bad := closedTextPipe(t)
			cmd := command(args...)
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if mode == "help" || mode == "result" || mode == "json diagnostic" {
				cmd.Stdout = bad
			}
			if mode == "diagnostic" || mode == "progress" || mode == "quiet" || mode == "json diagnostic" {
				cmd.Stderr = bad
			}
			done := startJSONProcess(t, cmd)
			bad.Close()
			waitJSONExit(t, done, code)
			switch mode {
			case "help":
				if !strings.Contains(errOut.String(), "output failed") {
					t.Fatalf("missing help failure: %q", &errOut)
				}
			case "progress":
				if out.Len() != 0 {
					t.Fatalf("failed progress emitted success: %q", &out)
				}
				data, err := os.ReadFile(path + storage.PartSuffix)
				if err != nil || string(data) != "abc" {
					t.Fatalf("broken stderr lost saved bytes: %q, %v", data, err)
				}
			case "result", "quiet":
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "abcdefghij" {
					t.Fatalf("verified output = %q, %v", data, err)
				}
				if mode == "result" && !strings.Contains(errOut.String(), "Verified output published: "+path) {
					t.Fatalf("failed path output hid publication: %q", &errOut)
				}
				if mode == "quiet" && out.String() != path+"\n" {
					t.Fatalf("quiet output = %q", &out)
				}
			}
		})
	}
}

func TestProcessTextCancelsFullStderr(t *testing.T) {
	seen := make(chan struct{})
	srv := textServer(t, seen)
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	textCheckpoint(t, path, srv.URL)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	filled := fillJSONPipe(t, writer)
	cmd := command("download", srv.URL, "-o", path, "-c", "1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, writer
	done := startJSONProcess(t, cmd)
	writer.Close()
	waitJSONState(t, done, func() bool {
		select {
		case <-seen:
			return true
		default:
			return false
		}
	})
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitJSONState(t, done, func() bool {
		f, err := storage.Open(context.Background(), path, false, true)
		if err != nil {
			return false
		}
		f.Close()
		return true
	})
	if _, err := io.CopyN(io.Discard, reader, int64(filled)); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	waitJSONExit(t, done, 130)
	if out.Len() != 0 || !bytes.Contains(data, []byte("download interrupted")) || !bytes.Contains(data, []byte("Partial file preserved:")) {
		t.Fatalf("blocked-stderr interruption: stdout=%q, stderr=%q", &out, data)
	}
	part, err := os.ReadFile(path + storage.PartSuffix)
	if err != nil || string(part) != "abc" {
		t.Fatalf("interruption changed saved bytes: %q, %v", part, err)
	}
}

func TestProcessTextMergedStreams(t *testing.T) {
	srv := textServer(t, nil)
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	cmd := command("download", srv.URL, "-o", path, "-c", "1")
	cmd.Stdout, cmd.Stderr = writer, writer
	done := startJSONProcess(t, cmd)
	writer.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	waitJSONExit(t, done, 0)
	if !bytes.HasSuffix(data, []byte(path+"\n")) || bytes.ContainsAny(data, "\x1b\r") || !bytes.Contains(data, []byte("0 workers")) {
		t.Fatalf("merged output = %q", data)
	}
}

func textServer(t *testing.T, seen chan struct{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			close(seen)
		}
		w.Header().Set("ETag", `"v1"`)
		body := "abcdefghij"
		if r.Header.Get("Range") != "" {
			if r.Header.Get("Range") != "bytes=3-" || r.Header.Get("If-Range") != `"v1"` {
				t.Error("text fixture lost its saved identity")
			}
			w.Header().Set("Content-Range", "bytes 3-9/10")
			w.WriteHeader(http.StatusPartialContent)
			body = body[3:]
		}
		fmt.Fprint(w, body)
	}))
}

func textCheckpoint(t *testing.T, path, raw string) {
	t.Helper()
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
}

func closedTextPipe(t *testing.T) *os.File {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	t.Cleanup(func() { writer.Close() })
	return writer
}
