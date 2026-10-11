package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidOptionsDoNotEchoValues(t *testing.T) {
	const secret = "https://user:private-password@example.org/file?token=private-token"

	for _, name := range []string{"connections", "c", "retries", "limit", "timeout", "quiet", "json", "resume", "overwrite"} {
		for _, inline := range []bool{false, true} {
			boolean := name == "quiet" || name == "json" || name == "resume" || name == "overwrite"
			if boolean && !inline {
				continue
			}

			t.Run(fmt.Sprintf("%s/inline=%v", name, inline), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "file")
				args := []string{"download", "https://example.org/file", "-o", path}

				if inline {
					args = append(args, "--"+name+"="+secret)
				} else {
					args = append(args, "--"+name, secret)
				}

				var out, errOut bytes.Buffer
				code := Run(context.Background(), args, &out, &errOut, "test")
				if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "--"+name) || strings.Contains(errOut.String(), "private-") {
					t.Fatalf("unsafe option diagnostic: code=%d, stdout=%q, stderr=%q", code, &out, &errOut)
				}

				if _, err := os.Stat(path + ".yoinkdl.lock"); !os.IsNotExist(err) {
					t.Fatal("invalid option started transfer work")
				}
			})
		}
	}
}
