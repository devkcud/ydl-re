package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "Usage: yoinkdl"},
		{[]string{"help"}, "Usage: yoinkdl"},
		{[]string{"--help"}, "Usage: yoinkdl"},
		{[]string{"help", "download"}, "--timeout"},
		{[]string{"download", "--help"}, "--output"},
		{[]string{"version"}, "yoinkdl test-version\n"},
	}

	for _, test := range cases {
		var out, errOut bytes.Buffer
		code := Run(context.Background(), test.args, &out, &errOut, "test-version")

		if code != 0 || !strings.Contains(out.String(), test.want) || errOut.Len() != 0 {
			t.Errorf("%v: code = %d, out = %q, err = %q", test.args, code, &out, &errOut)
		}
	}
}

func TestDownloadOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "file contents")
	}))
	defer srv.Close()

	for _, order := range []string{"url-first", "options-first", "mixed", "default-name"} {
		t.Run(order, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "chosen.bin")
			url := srv.URL + "/file.bin"
			args := []string{"download", url, "--quiet", "--output", path}

			switch order {
			case "options-first":
				args = []string{"download", "--quiet", "-o", path, url}
			case "mixed":
				args = []string{"download", "--output=" + path, url, "--quiet", "--timeout", "5s"}
			case "default-name":
				t.Chdir(dir)
				path = filepath.Join(dir, "file.bin")
				args = []string{"download", "--quiet", "--", url}
			}

			var out, errOut bytes.Buffer
			code := Run(context.Background(), args, &out, &errOut, "test")

			if code != 0 || out.String() != path+"\n" || errOut.Len() != 0 {
				t.Fatalf("code = %d, out = %q, err = %q", code, &out, &errOut)
			}

			data, err := os.ReadFile(path)
			if err != nil || string(data) != "file contents" {
				t.Fatalf("downloaded data = %q, error = %v", data, err)
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	cases := [][]string{
		{"missing"},
		{"version", "extra"},
		{"download"},
		{"download", "https://example.org", "extra"},
		{"download", "https://example.org", "--resume"},
		{"download", "https://example.org", "--connections", "4"},
		{"download", "https://example.org", "--output"},
		{"download", "https://example.org", "--output="},
		{"download", "https://example.org", "--timeout", "0s"},
		{"download", "https://example.org", "--timeout", "-1s"},
		{"download", "https://example.org", "--timeout", "invalid"},
	}

	for _, args := range cases {
		var out, errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut, "test")

		if code != 2 || out.Len() != 0 || errOut.Len() == 0 {
			t.Errorf("%v: code = %d, out = %q, err = %q", args, code, &out, &errOut)
		}
	}
}

func TestProgress(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprint(unknown), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if unknown {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}

				fmt.Fprint(w, "contents")
			}))
			defer srv.Close()

			path := filepath.Join(t.TempDir(), "file")
			var out, errOut bytes.Buffer
			code := Run(context.Background(), []string{"download", srv.URL, "-o", path}, &out, &errOut, "test")
			progress := errOut.String()

			if code != 0 || out.String() != path+"\n" || !strings.Contains(progress, "1 worker") {
				t.Fatalf("code = %d, out = %q, err = %q", code, &out, &errOut)
			}

			if unknown && (strings.Contains(progress, "%") || strings.Contains(progress, "ETA")) {
				t.Fatalf("invented a total: %q", progress)
			}

			if !unknown && !strings.Contains(progress, "100.0%") {
				t.Fatalf("missing final progress: %q", progress)
			}
		})
	}
}

func TestFailureAndInterruption(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", srv.URL, "-o", path, "--quiet"}, &out, &errOut, "test")

	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "HTTP 404") {
		t.Fatalf("failure: code = %d, out = %q, err = %q", code, &out, &errOut)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	errOut.Reset()
	code = Run(ctx, []string{"download", srv.URL, "-o", path}, &out, &errOut, "test")

	if code != 130 || out.Len() != 0 || !strings.Contains(errOut.String(), "interrupted") {
		t.Fatalf("interrupt: code = %d, out = %q, err = %q", code, &out, &errOut)
	}
}

func TestOverwrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "new")
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{
		"download", srv.URL, "-o", path, "--overwrite", "--quiet",
	}, &out, &errOut, "test")

	if code != 0 {
		t.Fatalf("overwrite failed: %s", &errOut)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("data = %q, error = %v", data, err)
	}
}
