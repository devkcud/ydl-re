//go:build !linux

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDaemonCLIUnsupportedPlatform(t *testing.T) {
	for _, args := range [][]string{{"daemon", "start"}, {"list"}, {"daemon", "stop"}} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut, "test"); code != 1 || !strings.Contains(errOut.String(), "require Linux") {
			t.Fatalf("unsupported command: code=%d, stdout=%q, stderr=%q", code, &out, &errOut)
		}
	}
}
