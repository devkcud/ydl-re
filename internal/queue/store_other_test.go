//go:build !linux

package queue

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreUnsupportedWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	s, err := Open(path)
	if s != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported storage was accepted: %v", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unsupported storage created files")
	}
}
