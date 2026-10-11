package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestJSONDownloadResults(t *testing.T) {
	for _, mode := range []string{"known", "unknown", "empty", "parallel"} {
		for _, quiet := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", mode, quiet), func(t *testing.T) {
				data := []byte("downloaded content")
				if mode == "empty" {
					data = nil
				} else if mode == "parallel" {
					data = bytes.Repeat([]byte("abc\x00"), int(ranges.MinChunkSize/4)+1)
				}
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if mode == "parallel" {
						var start, end int64
						if n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); n != 2 || err != nil {
							t.Error("missing JSON parallel range")
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						w.Header().Set("ETag", `"v1"`)
						w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
						w.WriteHeader(http.StatusPartialContent)
						w.Write(data[start : end+1])
						return
					}
					if mode == "unknown" {
						w.(http.Flusher).Flush()
					} else {
						w.Header().Set("Content-Length", strconv.Itoa(len(data)))
					}
					w.Write(data)
				}))
				defer srv.Close()
				path := filepath.Join(t.TempDir(), "quoted\"\nfile")
				args := []string{"download", "--json", srv.URL + "?token=private-secret", "-o", path}
				if quiet {
					args = append(args, "--quiet")
				}
				var out, errOut bytes.Buffer
				code := Run(context.Background(), args, &out, &errOut, "test")
				progress, result := jsonRecords(t, out.Bytes())
				if code != 0 || errOut.Len() != 0 || result.Status != "completed" || !result.Published || result.Error != nil ||
					result.Path == nil || *result.Path != path || result.Partial != nil || result.Bytes != int64(len(data)) ||
					result.Resumed != 0 || (len(progress) == 0) != quiet || strings.Contains(out.String(), "private-secret") {
					t.Fatalf("JSON success: code=%d, result=%+v, stderr=%q", code, result, &errOut)
				}
				if mode == "unknown" {
					if result.Total != nil {
						t.Fatal("invented an expected total")
					}
				} else if result.Total == nil || *result.Total != int64(len(data)) {
					t.Fatalf("incorrect expected total: %v", result.Total)
				}
				got, err := os.ReadFile(path)
				if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
					t.Fatalf("JSON output fixture mismatch: %v", err)
				}
			})
		}
	}
}

func TestJSONFailureAndSequentialResume(t *testing.T) {
	const data = "abcdefghijklmnopqrst"
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			fmt.Fprint(w, data[:5])
			return
		}
		if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"v1"` {
			t.Error("JSON resume lost its checkpoint identity")
		}
		w.Header().Set("Content-Range", "bytes 5-19/20")
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, data[5:])
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	args := []string{"download", srv.URL, "-o", path, "-c", "1", "--json"}
	var out, errOut bytes.Buffer
	code := Run(context.Background(), args, &out, &errOut, "test")
	_, result := jsonRecords(t, out.Bytes())
	if code != 1 || errOut.Len() != 0 || result.Status != "failed" || result.Published || result.Error == nil ||
		result.Bytes != 5 || result.Partial == nil || *result.Partial != path+storage.PartSuffix {
		t.Fatalf("JSON failure: code=%d, result=%+v, stderr=%q", code, result, &errOut)
	}
	out.Reset()
	code = Run(context.Background(), append(args, "--resume"), &out, &errOut, "test")
	progress, result := jsonRecords(t, out.Bytes())
	if code != 0 || errOut.Len() != 0 || result.Bytes != 20 || result.Resumed != 5 || !result.Published ||
		len(progress) == 0 || progress[0].Bytes != 5 || progress[0].Resumed != 5 || calls.Load() != 2 {
		t.Fatalf("JSON resume: code=%d, result=%+v, progress=%+v, stderr=%q", code, result, progress, &errOut)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != data {
		t.Fatalf("resumed data = %q, %v", got, err)
	}
}

func TestJSONRangeResumeWithRetry(t *testing.T) {
	const data = "abcdefghij"
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		if n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); n != 2 || err != nil ||
			r.Header.Get("If-Range") != `"v1"` || !((start == 0 && end == 0) || (start == 3 && end == 5) || (start == 9 && end == 9)) {
			t.Error("JSON range resume fetched saved coverage or lost its identity")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/10", start, end))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		if start == 3 && attempts.Add(1) == 1 {
			fmt.Fprint(w, "X")
			return
		}
		fmt.Fprint(w, data[start:end+1])
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(srv.URL)))
	if err := f.BeginRanges(storage.Manifest{SourceID: id, EffectiveID: id, Length: 10, ChunkSize: 3, ETag: `"v1"`, Encoding: "identity"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+storage.PartSuffix, []byte("abc???ghi?"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, span := range []ranges.Span{{Start: 0, End: 3}, {Start: 6, End: 9}} {
		sum := sha256.Sum256([]byte(data[span.Start:span.End]))
		if err := f.CheckpointRange(context.Background(), storage.Chunk{Span: span, Checksum: fmt.Sprintf("%x", sum)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", srv.URL, "-o", path, "--json", "--resume"}, &out, &errOut, "test")
	progress, result := jsonRecords(t, out.Bytes())
	if code != 0 || errOut.Len() != 0 || result.Bytes != 10 || result.Resumed != 6 || !result.Published || attempts.Load() != 2 ||
		len(progress) == 0 || progress[0].Bytes != 6 || progress[0].Resumed != 6 || progress[0].Workers != 0 {
		t.Fatalf("JSON range resume: code=%d, result=%+v, progress=%+v, stderr=%q", code, result, progress, &errOut)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != data {
		t.Fatalf("range-resumed data = %q, %v", got, err)
	}
}

func TestJSONPublicationRefusalAndOfflineRecovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content")
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	var out, errOut bytes.Buffer
	wrote := false
	writer := writeFunc(func(p []byte) (int, error) {
		if !wrote {
			wrote = true
			if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return out.Write(p)
	})
	args := []string{"download", srv.URL, "-o", path, "--json"}
	code := Run(context.Background(), args, writer, &errOut, "test")
	_, result := jsonRecords(t, out.Bytes())
	if code != 1 || errOut.Len() != 0 || result.Published || result.Status != "failed" || result.Partial == nil {
		t.Fatalf("publication refusal: code=%d, result=%+v, stderr=%q", code, result, &errOut)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "other" {
		t.Fatal("publication replaced the colliding file")
	}
	m, err := storage.Inspect(path)
	if err != nil || m.State != storage.StateReady {
		t.Fatalf("publication checkpoint = %+v, %v", m, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	out.Reset()
	code = Run(context.Background(), append(args, "--quiet"), &out, &errOut, "test")
	progress, result := jsonRecords(t, out.Bytes())
	if code != 0 || errOut.Len() != 0 || !result.Published || result.Resumed != 7 || result.Bytes != 7 || len(progress) != 0 {
		t.Fatalf("offline ready recovery: code=%d, result=%+v, stderr=%q", code, result, &errOut)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "content" {
		t.Fatalf("recovered data = %q, %v", got, err)
	}
}

func TestJSONOutputFailurePreservesTransferState(t *testing.T) {
	for _, phase := range []string{"initial", "partial", "terminal"} {
		t.Run(phase, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if phase != "partial" {
					fmt.Fprint(w, "content")
					return
				}
				w.Header().Set("Content-Length", "100")
				w.Header().Set("ETag", `"v1"`)
				fmt.Fprint(w, "abc")
				w.(http.Flusher).Flush()
				select {
				case <-time.After(progressInterval + 30*time.Millisecond):
					fmt.Fprint(w, "def")
					w.(http.Flusher).Flush()
				case <-r.Context().Done():
				}
				<-r.Context().Done()
			}))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file")
			var errOut bytes.Buffer
			failed := false
			writer := writeFunc(func(p []byte) (int, error) {
				if failed {
					t.Error("attempted another record after output failure")
				}
				var event resultEvent
				if err := json.Unmarshal(p, &event); err != nil {
					t.Fatal(err)
				}
				if phase == "partial" && event.Event == "progress" && event.Bytes == 0 {
					return len(p), nil
				}
				failed = true
				if phase == "terminal" {
					if event.Event != "result" || !event.Published {
						t.Fatal("quiet mode emitted nonterminal output")
					}
					if got, err := os.ReadFile(path); err != nil || string(got) != "content" {
						t.Fatal("result preceded verified publication")
					}
				}
				return 0, io.ErrClosedPipe
			})
			args := []string{"download", srv.URL, "-o", path, "--json", "-c", "1"}
			if phase == "terminal" {
				args = append(args, "--quiet")
			}
			code := Run(context.Background(), args, writer, &errOut, "test")
			if code != 1 || !failed || !strings.Contains(errOut.String(), "JSON output failed") {
				t.Fatalf("output failure: code=%d, stderr=%q", code, &errOut)
			}
			if phase == "terminal" {
				if !strings.Contains(errOut.String(), "Verified output published:") {
					t.Fatal("output failure hid successful publication")
				}
				if _, err := os.Stat(path + storage.ManifestSuffix); !os.IsNotExist(err) {
					t.Fatal("terminal event preceded manifest cleanup")
				}
			} else if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("published after progress output failed")
			}
			if phase == "partial" {
				if !strings.Contains(errOut.String(), "Partial file preserved:") {
					t.Fatal("output failure hid retained bytes")
				}
				m, err := storage.Inspect(path)
				if err != nil || m.Completed != 6 {
					t.Fatalf("retained checkpoint = %+v, %v", m, err)
				}
				f, err := storage.Open(context.Background(), path, false, true)
				if err != nil {
					t.Fatalf("output failure retained writer ownership: %v", err)
				}
				f.Close()
			}
		})
	}
}

func TestJSONArgumentErrorsAndTextMode(t *testing.T) {
	for _, args := range [][]string{
		{"download", "--json"},
		{"download", "--json", "http://example.org", "--limit", "0B/s"},
		{"download", "http://example.org", "--limit", "0B/s", "--json"},
		{"download", "http://example.org", "--json=invalid"},
		{"inspect", "file", "--json"},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut, "test"); code != 2 || out.Len() != 0 || errOut.Len() == 0 {
			t.Fatalf("JSON argument error: %v: code=%d, out=%q, stderr=%q", args, code, &out, &errOut)
		}
	}
	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", "--json", "--help"}, &out, &errOut, "test")
	if code != 0 || !strings.Contains(out.String(), "--json") || !strings.HasPrefix(out.String(), "Usage:") || errOut.Len() != 0 {
		t.Fatalf("JSON help: code=%d, out=%q, stderr=%q", code, &out, &errOut)
	}
	out.Reset()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	code = Run(context.Background(), []string{"download", srv.URL, "-o", path, "--json=false", "--quiet"}, &out, &errOut, "test")
	if code != 0 || out.String() != path+"\n" || errOut.Len() != 0 {
		t.Fatalf("explicit text mode: code=%d, out=%q, stderr=%q", code, &out, &errOut)
	}
}

func TestJSONEarlyFailuresDoNotInventState(t *testing.T) {
	for _, raw := range []string{"https://user:private-secret@example.org/file", "not a URL"} {
		var out, errOut bytes.Buffer
		code := Run(context.Background(), []string{"download", raw, "--json"}, &out, &errOut, "test")
		progress, result := jsonRecords(t, out.Bytes())
		if code != 1 || len(progress) != 0 || errOut.Len() != 0 || result.Published || result.Path != nil ||
			result.Partial != nil || result.Total != nil || strings.Contains(out.String(), "private-secret") {
			t.Fatalf("early refusal: code=%d, result=%+v, stderr=%q", code, result, &errOut)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	code := Run(ctx, []string{"download", "http://127.0.0.1:1/file", "--json"}, &out, &errOut, "test")
	_, result := jsonRecords(t, out.Bytes())
	if code != 130 || errOut.Len() != 0 || result.Status != "interrupted" || result.Path != nil || result.Published {
		t.Fatalf("early cancellation: code=%d, result=%+v, stderr=%q", code, result, &errOut)
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	path := filepath.Join(t.TempDir(), "file")
	errOut.Reset()
	code = Run(context.Background(), []string{"download", "http://127.0.0.1:1/file", "--json", "-o", path}, closed, &errOut, "test")
	if code != 1 || !strings.Contains(errOut.String(), "JSON output failed") {
		t.Fatalf("closed output initialization: code=%d, stderr=%q", code, &errOut)
	}
	if _, err := os.Stat(path + storage.LockSuffix); !os.IsNotExist(err) {
		t.Fatal("unusable JSON output started a transfer")
	}
}

func jsonRecords(t *testing.T, data []byte) ([]progressEvent, resultEvent) {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("missing newline-delimited records: %q", data)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	var progress []progressEvent
	var result resultEvent
	var elapsed int64
	for i, line := range lines {
		var event resultEvent
		if err := json.Unmarshal(line, &event); err != nil || event.Version != 1 || event.Elapsed < elapsed {
			t.Fatalf("invalid JSON event: %q, %v", line, err)
		}
		elapsed = event.Elapsed
		switch event.Event {
		case "progress":
			var snapshot progressEvent
			if err := json.Unmarshal(line, &snapshot); err != nil {
				t.Fatal(err)
			}
			progress = append(progress, snapshot)
		case "result":
			if i != len(lines)-1 {
				t.Fatal("terminal event was not last")
			}
			result = event
		default:
			t.Fatalf("unexpected event type %q", event.Event)
		}
	}
	if result.Event != "result" {
		t.Fatal("missing terminal result")
	}
	return progress, result
}
