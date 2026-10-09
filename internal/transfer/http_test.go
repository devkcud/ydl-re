package transfer

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestFilename(t *testing.T) {
	cases := map[string]string{
		"https://example.org/":                        "download",
		"https://example.org/file.zip?token=secret":   "file.zip",
		"https://example.org/a%2Fb.zip":               "a_b.zip",
		"https://example.org/%2E%2E":                  "download",
		"https://example.org/CON.txt":                 "_CON.txt",
		"https://example.org/file%0A.zip":             "file_.zip",
		"https://example.org/file.yoinkdl.part":       "file.yoinkdl.part.download",
		"https://example.org/file.yoinkdl.lock":       "file.yoinkdl.lock.download",
		"https://example.org/file.yoinkdl.tmp-abc":    "download",
		"https://example.org/%5C..%5Cfile%3Fname.zip": "_.._file_name.zip",
	}

	for raw, want := range cases {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}

		if got := filename(u); got != want {
			t.Errorf("filename(%q) = %q; want %q", raw, got, want)
		}
	}
}

func TestRedirectPolicy(t *testing.T) {
	prev, err := http.NewRequest(http.MethodGet, "https://example.org/file?token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://cdn.example.org/file", nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
		req.Header.Set(key, "secret")
	}

	if err := checkRedirect(req, []*http.Request{prev}); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
		if req.Header.Get(key) != "" {
			t.Errorf("forwarded %s across origins", key)
		}
	}

	req.Header.Set("Authorization", "secret")
	if err := checkRedirect(req, []*http.Request{prev, req}); err != nil {
		t.Fatal(err)
	}

	if req.Header.Get("Authorization") != "" {
		t.Fatal("restored original credentials on a subsequent redirect")
	}

	req.URL.Scheme = "http"
	if !errors.Is(checkRedirect(req, []*http.Request{prev}), errDowngrade) {
		t.Fatal("accepted HTTPS downgrade")
	}

	req.URL.Scheme = "https"
	req.URL.User = url.UserPassword("user", "secret")
	if !errors.Is(checkRedirect(req, []*http.Request{prev}), errRedirectTarget) {
		t.Fatal("accepted credentials in redirect target")
	}
}
