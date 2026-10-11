package transfer

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
	"sync/atomic"
	"testing"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func TestPartialRangeStateIsPreservedBeforeNetwork(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(w, "unsafe response")
			}))
			defer srv.Close()

			path, f := savedRange(t, srv.URL, 6)
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			before, err := os.ReadFile(path + storage.ManifestSuffix)
			if err != nil {
				t.Fatal(err)
			}

			result, err := Download(context.Background(), Request{
				URL: srv.URL, Output: path, Resume: explicit, Overwrite: true,
			}, nil)
			if err == nil || !strings.Contains(err.Error(), "parallel resume") || result.Bytes != 3 || result.Published || calls.Load() != 0 {
				t.Fatalf("range refusal = %+v, error = %v, requests = %d", result, err, calls.Load())
			}

			after, err := os.ReadFile(path + storage.ManifestSuffix)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("range refusal changed saved metadata")
			}

			assertData(t, path+storage.PartSuffix, "\x00\x00\x00def")
			assertAbsent(t, path)
		})
	}
}

func TestReadyRangeStatePublishesOffline(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	path, f := savedRange(t, url, 3)

	if err := os.WriteFile(path, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := f.Finalize(); err == nil {
		t.Fatal("expected publication collision")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	result, err := Download(context.Background(), Request{URL: url, Output: path, Resume: true}, nil)
	if err != nil || !result.Published || result.Resumed != 3 || result.Bytes != 3 {
		t.Fatalf("ready recovery = %+v, error = %v", result, err)
	}

	assertData(t, path, "def")
	assertAbsent(t, path+storage.PartSuffix)
	assertAbsent(t, path+storage.ManifestSuffix)
}

func savedRange(t *testing.T, raw string, length int64) (string, *storage.File) {
	t.Helper()
	u, err := parseURL(raw)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "file")
	f, err := storage.Create(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	err = f.BeginRanges(storage.Manifest{
		SourceID: fingerprint(u), EffectiveID: fingerprint(u), Length: length,
		ETag: `"v1"`, Encoding: "identity", ChunkSize: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(path+storage.PartSuffix, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if _, err := file.WriteAt([]byte("def"), length-3); err != nil {
		t.Fatal(err)
	}

	err = f.CheckpointRange(context.Background(), storage.Chunk{
		Span: ranges.Span{Start: length - 3, End: length}, Checksum: fmt.Sprintf("%x", sha256.Sum256([]byte("def"))),
	})
	if err != nil {
		t.Fatal(err)
	}

	return path, f
}
