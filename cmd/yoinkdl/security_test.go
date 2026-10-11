package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessDoesNotLogUnsolicitedResponseData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}

		defer conn.Close()
		fmt.Fprint(rw, "HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\nabcprivate-token-after-response")
		rw.Flush()
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	cmd := command("download", srv.URL, "-o", path, "-c", "1", "--quiet")

	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("process = %v, stderr=%q", err, &errOut)
	}
	if out.String() != path+"\n" || strings.Contains(errOut.String(), "private-token") || errOut.Len() != 0 {
		t.Fatalf("untrusted transport log: stdout=%q, stderr=%q", &out, &errOut)
	}
}
