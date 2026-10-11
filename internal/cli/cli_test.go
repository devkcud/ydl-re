package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
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
		{[]string{"download", "--help"}, "--limit"},
		{[]string{"help", "inspect"}, "checkpoint metadata"},
		{[]string{"inspect", "--help"}, "checkpoint metadata"},
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
		{"inspect"},
		{"inspect", "first", "second"},
		{"download", "https://example.org", "--connections", "0"},
		{"download", "https://example.org", "--connections", "33"},
		{"download", "https://example.org", "--connections", "+4"},
		{"download", "https://example.org", "--connections", "0x4"},
		{"download", "https://example.org", "--output"},
		{"download", "https://example.org", "--output="},
		{"download", "https://example.org", "--timeout", "0s"},
		{"download", "https://example.org", "--timeout", "-1s"},
		{"download", "https://example.org", "--timeout", "invalid"},
		{"download", "https://example.org", "---quiet"},
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

func TestInspectReadsActiveCheckpointWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	err = f.Begin(storage.Manifest{
		SourceID:    strings.Repeat("a", 64),
		EffectiveID: strings.Repeat("b", 64),
		Length:      10,
		ETag:        `"version-1"`,
		Encoding:    "identity",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}

	if err := f.Checkpoint(true); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"inspect", path}, &out, &errOut, "test")

	if code != 0 || errOut.Len() != 0 || !strings.Contains(out.String(), "Saved bytes: 3") || !strings.Contains(out.String(), "Total bytes: 10") {
		t.Fatalf("inspect: code = %d, out = %q, err = %q", code, &out, &errOut)
	}

	after, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("inspection changed saved state")
	}
}

func TestResumeAndInspectMissingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	cases := [][]string{
		{"download", "https://example.org/file", "--output", path, "--resume"},
		{"inspect", path},
	}

	for _, args := range cases {
		var out, errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut, "test")

		if code != 1 || out.Len() != 0 || errOut.Len() == 0 {
			t.Fatalf("%v: code = %d, out = %q, err = %q", args, code, &out, &errOut)
		}
	}
}

func TestResumedBytesAreNotTransferSpeed(t *testing.T) {
	var out bytes.Buffer
	display := testProgress(t, &out)
	display.Update(context.Background(), transfer.Snapshot{Bytes: 4096, Total: 8192, Resumed: 4096})

	if !strings.Contains(out.String(), "50.0%") || !strings.Contains(out.String(), "0.0 B/s") {
		t.Fatalf("saved bytes counted as new traffic: %q", &out)
	}
}

func TestInspectRangeCoverage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	err = f.BeginRanges(storage.Manifest{
		SourceID: strings.Repeat("a", 64), EffectiveID: strings.Repeat("b", 64),
		Length: 10, ETag: `"v1"`, Encoding: "identity", ChunkSize: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path+storage.PartSuffix, []byte("abcBADghi"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, span := range []ranges.Span{{Start: 0, End: 3}, {Start: 6, End: 9}} {
		sum := sha256.Sum256([]byte("abcBADghi"[span.Start:span.End]))
		if err := f.CheckpointRange(context.Background(), storage.Chunk{Span: span, Checksum: fmt.Sprintf("%x", sum)}); err != nil {
			t.Fatal(err)
		}
	}

	before, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path+storage.PartSuffix, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"inspect", path}, &out, &errOut, "test")
	if code != 0 || errOut.Len() != 0 || !strings.Contains(out.String(), "Saved bytes: 6") ||
		!strings.Contains(out.String(), "Total bytes: 10") || !strings.Contains(out.String(), "eligible after local and server validation") {
		t.Fatalf("range inspect: code = %d, out = %q, err = %q", code, &out, &errOut)
	}

	after, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("inspection changed range state")
	}
}

func TestConnectionOptions(t *testing.T) {
	for _, test := range []struct {
		args []string
		want int
	}{
		{nil, transfer.DefaultConnections},
		{[]string{"-c", "1"}, 1},
		{[]string{"--connections=32"}, 32},
		{[]string{"--connections", "04"}, 4},
	} {
		req, _, err := parseDownload(append([]string{"https://example.org/file"}, test.args...))
		if err != nil || req.Connections != test.want {
			t.Fatalf("%v: connections = %d, error = %v", test.args, req.Connections, err)
		}
	}

	for _, value := range []string{"", "0", "-1", "33", "+4", "0x4", "1.5", "99999999999999999999"} {
		if _, _, err := parseDownload([]string{"https://example.org/file", "-c", value}); err == nil {
			t.Errorf("accepted invalid connection limit %q", value)
		}
	}
}

func TestOneConnectionUsesDirectStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			t.Error("one-connection mode sent a capability probe")
		}
		fmt.Fprint(w, "stream")
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"download", srv.URL, "-c", "1", "-o", path, "--quiet"}, &out, &errOut, "test")
	if code != 0 || out.String() != path+"\n" || errOut.Len() != 0 {
		t.Fatalf("direct stream: code = %d, out = %q, error = %q", code, &out, &errOut)
	}
}

func TestProgressWorkerCount(t *testing.T) {
	for _, count := range []int{0, 1, 4} {
		var out bytes.Buffer
		p := testProgress(t, &out)
		p.Update(context.Background(), transfer.Snapshot{Bytes: 3, Total: 10, Workers: count})
		p.Finish()
		if !strings.Contains(out.String(), fmt.Sprintf("%d worker", count)) {
			t.Errorf("missing active worker count %d: %q", count, &out)
		}
	}
}

func TestRetryOptions(t *testing.T) {
	for _, test := range []struct {
		args []string
		want int
	}{
		{nil, transfer.DefaultRetries},
		{[]string{"--retries", "0"}, 0},
		{[]string{"--retries=10"}, 10},
		{[]string{"--retries", "03"}, 3},
	} {
		req, _, err := parseDownload(append([]string{"https://example.org/file"}, test.args...))
		if err != nil || req.Retries == nil || *req.Retries != test.want {
			t.Fatalf("%v: retries=%v, error=%v", test.args, req.Retries, err)
		}
	}
	for _, value := range []string{"", "-1", "11", "+3", "0x3", "1.5", "99999999999999999999"} {
		if _, _, err := parseDownload([]string{"https://example.org/file", "--retries", value}); err == nil {
			t.Errorf("accepted invalid retries %q", value)
		}
	}
}

func TestRateOptions(t *testing.T) {
	for _, test := range []struct {
		args []string
		want int64
	}{
		{nil, 0},
		{[]string{"--limit", "5MB/s"}, 5000000},
		{[]string{"--limit=5MiB/s"}, 5242880},
		{[]string{"--limit", "0001B/s"}, 1},
	} {
		for _, first := range []bool{false, true} {
			args := append([]string{"https://example.org/file"}, test.args...)
			if first {
				args = append(append([]string{}, test.args...), "https://example.org/file")
			}
			req, _, err := parseDownload(args)
			if err != nil || req.Limit != test.want {
				t.Fatalf("%v: limit=%d, error=%v", args, req.Limit, err)
			}
		}
	}
	for _, value := range []string{"", "0B/s", "-1B/s", "1.5MB/s", "1MB", " 1MB/s", "1mb/s", "9223372036854775808B/s"} {
		path := filepath.Join(t.TempDir(), "file")
		var out, errOut bytes.Buffer
		code := Run(context.Background(), []string{"download", "http://127.0.0.1:1/file", "-o", path, "--limit", value}, &out, &errOut, "test")
		if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "limit") {
			t.Fatalf("invalid rate: code=%d, out=%q, error=%q", code, &out, &errOut)
		}
		for _, suffix := range []string{"", storage.PartSuffix, storage.ManifestSuffix, storage.LockSuffix} {
			if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
				t.Fatalf("invalid rate touched %s: %v", suffix, err)
			}
		}
	}
}
