package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"yoinkdl/internal/transfer"
)

func TestTextPathRendering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer srv.Close()
	for _, name := range []string{"ação with spaces.txt", "line\nquote\"\x1b[31m\u202e"} {
		t.Run(strconv.Quote(name), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			var out, errOut bytes.Buffer
			code := Run(context.Background(), []string{"download", srv.URL, "-o", path, "--quiet"}, &out, &errOut, "test")
			want := path + "\n"
			if strings.ContainsRune(name, '\n') {
				want = strconv.Quote(path) + "\n"
			}
			if code != 0 || out.String() != want || errOut.Len() != 0 {
				t.Fatalf("path rendering: code=%d, stdout=%q, stderr=%q; want %q", code, &out, &errOut, want)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "ok" {
				t.Fatalf("display changed the actual filename: %q, %v", got, err)
			}
		})
	}
}

func TestTextDiagnosticsEscapeControls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing\n\x1b[31m\u202e", "file")
	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", "http://127.0.0.1:1/file", "-o", path, "--quiet"}, &out, &errOut, "test")
	text := errOut.String()
	if code != 1 || out.Len() != 0 || strings.Count(text, "\n") != 1 || strings.ContainsAny(text, "\x1b\u202e") {
		t.Fatalf("unsafe diagnostic: code=%d, stdout=%q, stderr=%q", code, &out, text)
	}
	if !strings.Contains(text, `\n`) || !strings.Contains(text, `\x1b`) || !strings.Contains(text, `\u202e`) {
		t.Fatalf("diagnostic lost the escaped path: %q", text)
	}
	errOut.Reset()
	code = Run(context.Background(), []string{"version"}, &out, &errOut, "build\n\x1b[31m")
	if code != 0 || out.String() != "yoinkdl \"build\\n\\x1b[31m\"\n" {
		t.Fatalf("unsafe build label: code=%d, stdout=%q", code, &out)
	}
}

func TestProgressRedirectedFrames(t *testing.T) {
	var out bytes.Buffer
	p := newProgress(&out)
	p.Update(transfer.Snapshot{Bytes: 2, Total: 10, Workers: 2})
	p.Finish()
	text := out.String()
	if strings.ContainsAny(text, "\r\x1b") {
		t.Fatalf("redirected progress contained terminal controls: %q", text)
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "2 workers") || !strings.Contains(lines[1], "0 workers") {
		t.Fatalf("final progress retained active workers: %q", text)
	}
}

func TestProgressCharacterDeviceIsNotAlwaysTerminal(t *testing.T) {
	file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if p := newProgress(file); p.inline {
		t.Fatal("treated the null device as an interactive terminal")
	}
}
