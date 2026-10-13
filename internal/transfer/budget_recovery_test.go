package transfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"yoinkdl/internal/storage"
)

func TestSharedBudgetReadyRecoveryIsOfflineAndFree(t *testing.T) {
	for _, mode := range []string{"sequential", "ranges"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const raw = "http://127.0.0.1:1/file"
				var path string
				var f *storage.File
				if mode == "ranges" {
					path, f = savedRange(t, raw, 3)
				} else {
					path = filepath.Join(t.TempDir(), "file")
					var err error
					f, err = storage.Create(path, false)
					if err != nil {
						t.Fatal(err)
					}
					u, _ := parseURL(raw)
					if err := f.Begin(storage.Manifest{SourceID: fingerprint(u), EffectiveID: fingerprint(u), Length: 3, Encoding: "identity"}); err != nil {
						t.Fatal(err)
					}
					if _, err := f.Write([]byte("def")); err != nil {
						t.Fatal(err)
					}
				}
				defer f.Close()
				if err := os.WriteFile(path, []byte("collision"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := f.Finalize(); err == nil {
					t.Fatal("expected publication conflict")
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				budget, _ := NewBudget(1)
				if err := waitBudgets(context.Background(), 1, budget); err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				result, err := budget.Download(context.Background(), Request{URL: raw, Output: path, Resume: true, Limit: 1}, nil)
				if err != nil || !result.Published || result.Resumed != 3 || result.Bytes != 3 || time.Since(start) != 0 {
					t.Fatalf("offline recovery = %+v, %v, elapsed=%s", result, err, time.Since(start))
				}
				assertData(t, path, "def")
				if err := waitBudgets(context.Background(), 1, budget); err != nil {
					t.Fatal(err)
				}
				if time.Since(start) != time.Second {
					t.Fatalf("offline recovery charged or reset the shared budget: elapsed=%s", time.Since(start))
				}
			})
		})
	}
}
