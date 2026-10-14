//go:build !linux

package daemon

import (
	"context"
	"errors"
	"testing"
)

func TestDaemonUnsupportedPlatform(t *testing.T) {
	if _, err := Resolve(); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported path resolution succeeded")
	}
	if _, err := New(Options{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported daemon started")
	}
	if _, err := Call(context.Background(), Paths{}, "ping", map[string]any{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported client connected")
	}
}
