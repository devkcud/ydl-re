package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestQueueHelpAndInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"help", "daemon"}, {"help", "daemon", "start"}, {"daemon", "stop", "--help"},
		{"daemon", "set", "--help"}, {"help", "add"}, {"add", "--help"}, {"list", "--help"},
		{"pause", "--help"}, {"resume", "--help"}, {"cancel", "--help"}, {"forget", "--help"},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut, "test"); code != 0 || !strings.Contains(out.String(), "Usage:") || errOut.Len() != 0 {
			t.Fatalf("help %v: code=%d, out=%q, err=%q", args, code, &out, &errOut)
		}
	}
	for _, args := range [][]string{
		{"daemon"}, {"daemon", "unknown"}, {"daemon", "start", "extra"}, {"daemon", "start", "--json"},
		{"daemon", "stop", "--limit", "1B/s"}, {"daemon", "set"}, {"daemon", "set", "--parallel", "0"},
		{"daemon", "set", "--parallel", "9"}, {"daemon", "start", "--parallel", "+2"},
		{"daemon", "start", "--limit", "0B/s"}, {"list", "extra"}, {"pause"}, {"resume", "BAD"},
		{"cancel", strings.Repeat("A", 32)}, {"forget", strings.Repeat("a", 32), "extra"},
		{"add"}, {"add", "https://example.org/", "extra"}, {"add", "https://example.org/", "--overwrite=false"},
		{"add", "https://example.org/", "--quiet"}, {"add", "https://example.org/", "--output="},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut, "test"); code != 2 || out.Len() != 0 || errOut.Len() == 0 {
			t.Fatalf("invalid %v: code=%d, out=%q, err=%q", args, code, &out, &errOut)
		}
	}
}

func TestQueueInvalidArgumentsNeverEchoSourceValues(t *testing.T) {
	const secret = "https://user:private-password@example.org/?token=private-token"
	for _, name := range []string{"connections", "c", "retries", "limit", "timeout", "resume", "json"} {
		var out, errOut bytes.Buffer
		args := []string{"add", "https://example.org/file", "--" + name + "=" + secret}
		if code := Run(context.Background(), args, &out, &errOut, "test"); code != 2 || strings.Contains(errOut.String(), "private-") {
			t.Fatalf("unsafe %s: code=%d, stderr=%q", name, code, &errOut)
		}
	}
	for _, args := range [][]string{
		{"add", secret}, {"daemon", secret}, {"help", "daemon", secret}, {"pause", secret},
		{"daemon", "start", "--parallel=" + secret}, {"daemon", "start", "--limit=" + secret},
		{"list", "--json=" + secret}, {"daemon", "stop", "--json=" + secret},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut, "test"); code != 2 || strings.Contains(fmt.Sprint(&out, &errOut), "private-") {
			t.Fatalf("unsafe arguments: code=%d, stdout=%q, stderr=%q", code, &out, &errOut)
		}
	}
}
