package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"yoinkdl/internal/storage"
)

func TestProcessJSONInterruptionAndResume(t *testing.T) {
	const prefix = 65536
	data := bytes.Repeat([]byte("abcd\x00xyz"), 1024*1024/8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		if r.Header.Get("Range") == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data[:prefix])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		if r.Header.Get("Range") != "bytes=65536-" || r.Header.Get("If-Range") != `"v1"` {
			t.Error("JSON process resumed from an incorrect identity or offset")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", prefix, len(data)-1, len(data)))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)-prefix))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[prefix:])
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	raw := srv.URL + "/file?token=private-secret"
	cmd := command("download", raw, "-o", path, "-c", "1", "--json")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	done := startJSONProcess(t, cmd)
	waitJSONState(t, done, func() bool {
		info, err := os.Stat(path + storage.PartSuffix)
		return err == nil && info.Size() >= prefix
	})
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	waitJSONExit(t, done, 130)
	result, _ := processJSONResult(t, out.Bytes())
	if errOut.Len() != 0 || result.Status != "interrupted" || result.ExitCode != 130 || result.Published ||
		result.Bytes != prefix || result.Partial != path+storage.PartSuffix || result.Error == nil ||
		strings.Contains(out.String(), "private-secret") {
		t.Fatalf("JSON interruption: result=%+v, stderr=%q", result, &errOut)
	}
	m, err := storage.Inspect(path)
	if err != nil || m.Completed != prefix || m.Checksum != fmt.Sprintf("%x", sha256.Sum256(data[:prefix])) {
		t.Fatalf("interrupted checkpoint = %+v, %v", m, err)
	}
	cmd = command("download", raw, "-o", path, "--json", "--quiet", "--resume")
	out.Reset()
	errOut.Reset()
	cmd.Stdout, cmd.Stderr = &out, &errOut
	waitJSONExit(t, startJSONProcess(t, cmd), 0)
	result, count := processJSONResult(t, out.Bytes())
	if count != 1 || errOut.Len() != 0 || result.Status != "completed" || !result.Published || result.Error != nil ||
		result.Resumed != prefix || result.Bytes != int64(len(data)) || result.Path != path || result.Partial != "" {
		t.Fatalf("JSON process resume: result=%+v, stderr=%q", result, &errOut)
	}
	got, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatalf("JSON process fixture mismatch: %v", err)
	}
}

type processEvent struct {
	Version   int     `json:"schema_version"`
	Event     string  `json:"event"`
	Status    string  `json:"status"`
	ExitCode  int     `json:"exit_code"`
	Path      string  `json:"path"`
	Partial   string  `json:"partial"`
	Bytes     int64   `json:"bytes"`
	Resumed   int64   `json:"resumed"`
	Published bool    `json:"published"`
	Error     *string `json:"error"`
}

func processJSONResult(t *testing.T, data []byte) (processEvent, int) {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("process output lacks a complete JSON line: %q", data)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	var result processEvent
	for i, line := range lines {
		var event processEvent
		if err := json.Unmarshal(line, &event); err != nil || event.Version != 1 {
			t.Fatalf("invalid process event: %q, %v", line, err)
		}
		if event.Event == "result" {
			if i != len(lines)-1 {
				t.Fatal("process emitted records after its terminal result")
			}
			result = event
		} else if event.Event != "progress" {
			t.Fatalf("unexpected process event %q", event.Event)
		}
	}
	if result.Event != "result" {
		t.Fatal("process omitted its terminal result")
	}
	return result, len(lines)
}

func startJSONProcess(t *testing.T, cmd *exec.Cmd) <-chan error {
	t.Helper()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			cmd.Process.Kill()
			<-done
		}
	})
	return done
}

func waitJSONExit(t *testing.T, done <-chan error, code int) {
	t.Helper()
	select {
	case err := <-done:
		if code == 0 && err == nil {
			return
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != code {
			t.Fatalf("process exit = %v; want %d", err, code)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("JSON process did not stop")
	}
}

func waitJSONState(t *testing.T, done <-chan error, ready func() bool) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		if ready() {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("JSON process exited before the expected state: %v", err)
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("JSON process did not reach the expected state")
		}
	}
}
