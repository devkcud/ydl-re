package transfer

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeRequest(t *testing.T) {
	got, err := Normalize(Request{URL: "https://example.invalid/a%2Fb.zip?token=secret#fragment"})
	if err != nil {
		t.Fatal(err)
	}

	if got.URL != "https://example.invalid/a%2Fb.zip?token=secret" || got.Output != "a_b.zip" ||
		got.Timeout != DefaultTimeout || got.Connections != DefaultConnections ||
		got.Retries == nil || *got.Retries != DefaultRetries || got.Limit != 0 {
		t.Fatal("normalization changed URL identity, filename rules, or defaults")
	}

	again, err := Normalize(got)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("request normalization is not idempotent")
	}
}

func TestNormalizeOwnsExplicitOptionsWithoutFileWork(t *testing.T) {
	dir := t.TempDir()
	retries := 0
	req := Request{
		URL: "https://example.invalid/file", Output: filepath.Join(dir, "missing", "file"),
		Timeout: time.Nanosecond, Connections: 1, Retries: &retries, Limit: 1, Overwrite: true, Resume: true,
	}

	got, err := Normalize(req)
	if err != nil || !reflect.DeepEqual(got, req) || got.Retries == req.Retries {
		t.Fatal("normalization did not preserve and own explicit options")
	}

	retries = 5
	if *got.Retries != 0 {
		t.Fatal("normalized options alias caller retry storage")
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("normalization created filesystem state")
	}

	req.Output = "./keep/../exact\nname"
	got, err = Normalize(req)
	if err != nil || got.Output != req.Output {
		t.Fatal("normalization changed an explicit output path")
	}
}

func TestNormalizeInvalidRequestsMatchDownload(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Request)
		want string
	}{
		{"relative", func(r *Request) { r.URL = "/file?token=secret" }, "provide an absolute HTTP or HTTPS URL"},
		{"credentials", func(r *Request) { r.URL = "https://user:secret@example.invalid/file" }, "credentials in URLs are not supported"},
		{"escape", func(r *Request) { r.URL = "https://example.invalid/%zz?token=secret" }, "provide an absolute HTTP or HTTPS URL"},
		{"timeout", func(r *Request) { r.Timeout = -1 }, "timeout must be positive"},
		{"workers low", func(r *Request) { r.Connections = -1 }, "connections must be between 1 and 32"},
		{"workers high", func(r *Request) { r.Connections = 33 }, "connections must be between 1 and 32"},
		{"retries low", func(r *Request) {
			n := -1
			r.Retries = &n
		}, "retries must be between 0 and 10"},
		{"retries high", func(r *Request) {
			n := 11
			r.Retries = &n
		}, "retries must be between 0 and 10"},
		{"limit", func(r *Request) { r.Limit = -1 }, "limit must not be negative"},
		{"ordering", func(r *Request) {
			r.Timeout = -1
			r.Connections = -1
			r.Limit = -1
		}, "timeout must be positive"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := Request{URL: "https://example.invalid/file?token=secret", Output: filepath.Join(t.TempDir(), "file")}
			test.edit(&req)
			got, err := Normalize(req)
			if err == nil || err.Error() != test.want || strings.Contains(err.Error(), "secret") || !reflect.DeepEqual(got, Request{}) {
				t.Fatalf("normalization error = %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = Download(ctx, req, nil)
			if err == nil || err.Error() != test.want {
				t.Fatalf("download validation changed ordering or diagnostics: %v", err)
			}
		})
	}
}
