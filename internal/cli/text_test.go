package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

func TestTextRepliesDetectWriteFailures(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"help", "download"}, {"help", "inspect"},
		{"download", "--help"}, {"inspect", "--help"}, {"version"}} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%v", args, short), func(t *testing.T) {
				writer := writeFunc(func(p []byte) (int, error) {
					if short {
						return len(p) - 1, nil
					}
					return 0, io.ErrClosedPipe
				})
				var errOut bytes.Buffer
				if code := Run(context.Background(), args, writer, &errOut, "test"); code != 1 || !strings.Contains(errOut.String(), "output failed") {
					t.Fatalf("reply write failed without an error result: code=%d, stderr=%q", code, &errOut)
				}
			})
		}
	}
}

func TestTextProgressFailureStopsDownload(t *testing.T) {
	for _, short := range []bool{false, true} {
		t.Run(fmt.Sprint(short), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "content") }))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file")
			var out bytes.Buffer
			writer := writeFunc(func(p []byte) (int, error) {
				if short {
					return len(p) - 1, nil
				}
				return 0, io.ErrClosedPipe
			})
			code := Run(context.Background(), []string{"download", srv.URL, "-o", path, "-c", "1"}, &out, writer, "test")
			if code != 1 || out.Len() != 0 {
				t.Fatalf("progress failure reported success: code=%d, stdout=%q", code, &out)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("published after progress output failed")
			}
		})
	}
}

func TestTextResultFailureReportsPublishedPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "content") }))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	writer := writeFunc(func([]byte) (int, error) { return 0, io.ErrClosedPipe })
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", srv.URL, "-o", path, "--quiet"}, writer, &errOut, "test")
	if code != 1 || !strings.Contains(errOut.String(), "Verified output published: "+path) {
		t.Fatalf("output failure hid publication: code=%d, stderr=%q", code, &errOut)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "content" {
		t.Fatalf("output failure lost verified file: %q, %v", data, err)
	}
}

func TestTextInspectWriteFailureIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Begin(storage.Manifest{SourceID: strings.Repeat("a", 64), EffectiveID: strings.Repeat("b", 64), Length: 10, ETag: `"v1"`, Encoding: "identity"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil {
		t.Fatal(err)
	}
	writer := writeFunc(func(p []byte) (int, error) { return len(p) - 1, nil })
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"inspect", path}, writer, &errOut, "test")
	if code != 1 || !strings.Contains(errOut.String(), "output failed") {
		t.Fatalf("inspection ignored a short write: code=%d, stderr=%q", code, &errOut)
	}
	after, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("inspection output failure changed an active checkpoint")
	}
}

func TestTextFailurePreservesCheckpointOrPublication(t *testing.T) {
	for _, final := range []bool{false, true} {
		t.Run(fmt.Sprint(final), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if final {
					fmt.Fprint(w, "content")
					return
				}
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Length", "100")
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
			var out, errOut bytes.Buffer
			calls := 0
			writer := writeFunc(func(p []byte) (int, error) {
				calls++
				if calls == 2 {
					return 0, io.ErrClosedPipe
				}
				return errOut.Write(p)
			})
			code := Run(context.Background(), []string{"download", srv.URL, "-o", path, "-c", "1"}, &out, writer, "test")
			if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "write progress") || calls != 3 {
				t.Fatalf("progress failure: code=%d, stdout=%q, stderr=%q, writes=%d", code, &out, &errOut, calls)
			}
			if final {
				if data, err := os.ReadFile(path); err != nil || string(data) != "content" || !strings.Contains(errOut.String(), "Verified output published:") {
					t.Fatalf("final progress failure hid publication: %q, %v, %s", data, err, &errOut)
				}
			} else {
				m, err := storage.Inspect(path)
				if err != nil || m.Completed != 6 || !strings.Contains(errOut.String(), "Partial file preserved:") {
					t.Fatalf("output failure lost checkpoint: %+v, %v, %s", m, err, &errOut)
				}
				f, err := storage.Open(context.Background(), path, false, true)
				if err != nil {
					t.Fatalf("output failure retained ownership: %v", err)
				}
				f.Close()
			}
		})
	}
}

func TestTextReplyAndDiagnosticCancellation(t *testing.T) {
	for _, help := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			writer, reader := net.Pipe()
			defer writer.Close()
			defer reader.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var other bytes.Buffer
			done := make(chan int, 1)
			go func() {
				if help {
					done <- Run(ctx, []string{"help"}, writer, &other, "test")
				} else {
					done <- Run(ctx, []string{"unknown"}, &other, writer, "test")
				}
			}()
			synctest.Wait()
			cancel()
			if code := <-done; code != 130 {
				t.Fatalf("blocked command ignored cancellation: %d", code)
			}
		})
	}
}

func TestTextUnusedAndFailedDiagnostics(t *testing.T) {
	writer := writeFunc(func([]byte) (int, error) { return 0, io.ErrClosedPipe })
	for _, args := range [][]string{{"unknown"}, {"download"}, {"version", "extra"}} {
		var out bytes.Buffer
		if code := Run(context.Background(), args, &out, writer, "test"); code != 2 || out.Len() != 0 {
			t.Fatalf("failed diagnostic lost command status: %v: %d, %q", args, code, &out)
		}
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	path := filepath.Join(t.TempDir(), "file")
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", "http://127.0.0.1:1/file", "-o", path, "--quiet"}, closed, &errOut, "test")
	if code != 1 || !strings.Contains(errOut.String(), "prepare stdout") {
		t.Fatalf("closed stdout initialization: code=%d, stderr=%q", code, &errOut)
	}
	if _, err := os.Stat(path + storage.LockSuffix); !os.IsNotExist(err) {
		t.Fatal("closed stdout started transfer work")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer srv.Close()
	var out bytes.Buffer
	code = Run(context.Background(), []string{"download", srv.URL, "-o", path, "--quiet"}, &out, closed, "test")
	if code != 0 || out.String() != path+"\n" {
		t.Fatalf("quiet mode required unused stderr: %d, %q", code, &out)
	}
}

func TestTextProgressCoalescesAfterBackpressure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		p := testProgress(t, writeFunc(func(data []byte) (int, error) {
			calls++
			time.Sleep(time.Second)
			return len(data), nil
		}))
		for range 2 {
			if err := p.Update(context.Background(), transfer.Snapshot{Total: 10, Workers: 1}); err != nil {
				t.Fatal(err)
			}
		}
		if calls != 1 {
			t.Fatal("slow writes bypassed progress coalescing")
		}
		p.Finish()
	})
}
