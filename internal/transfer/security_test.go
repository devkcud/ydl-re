package transfer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yoinkdl/internal/storage"
)

func TestSequentialTrailerErrorsDoNotEchoServerData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}

		defer conn.Close()
		fmt.Fprint(rw, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nTrailer: X-Check\r\n\r\n3\r\nabc\r\n0\r\nprivate-token-without-colon\r\n\r\n")
		rw.Flush()
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Connections: 1}, nil)
	if err == nil || result.Published || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("unsafe trailer error: result=%+v, error=%v", result, err)
	}

	assertAbsent(t, path)
	assertData(t, path+".yoinkdl.part", "abc")
}

func TestSingleStreamRejectsContradictoryFullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 0-2/10")
		fmt.Fprint(w, "abc")
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Connections: 1}, nil)
	if err == nil || result.Published {
		t.Fatalf("accepted contradictory full response: %+v, %v", result, err)
	}

	assertAbsent(t, path)
}

func TestZeroPrefixRejectsContradictoryFullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version"`)
		w.Header().Set("Content-Range", "bytes 0-9/10")
		fmt.Fprint(w, "abcdefghij")
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	m := storage.Manifest{SourceID: fingerprint(u), EffectiveID: fingerprint(u), Length: 10, ETag: `"version"`, Encoding: "identity"}
	if err := f.Begin(m); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("saved-tail")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path + storage.ManifestSuffix)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Download(context.Background(), Request{URL: srv.URL, Output: path, Connections: 1}, nil)
	if err == nil || result.Published {
		t.Fatalf("accepted contradictory saved response: %+v, %v", result, err)
	}

	assertAbsent(t, path)
	assertData(t, path+storage.PartSuffix, "saved-tail")
	assertData(t, path+storage.ManifestSuffix, string(before))
}
