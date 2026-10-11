package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLIProcess(t *testing.T) {
	if os.Getenv("YOINKDL_TEST_PROCESS") != "1" {
		return
	}

	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"yoinkdl"}, os.Args[i+1:]...)
			if os.Getenv("YOINKDL_TEST_CLOSE_STDERR") == "1" {
				os.Stderr.Close()
			}
			main()
			return
		}
	}

	t.Fatal("missing process arguments")
}

func TestExecutable(t *testing.T) {
	out, err := command("version").CombinedOutput()
	if err != nil || string(out) != "yoinkdl dev\n" {
		t.Fatalf("version = %q, error = %v", out, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "complete file")
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	out, err = command("download", srv.URL, "--output", path, "--quiet").CombinedOutput()
	if err != nil || string(out) != path+"\n" {
		t.Fatalf("download = %q, error = %v", out, err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "complete file" {
		t.Fatalf("output = %q, error = %v", data, err)
	}
}

func TestInterruptProcess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		fmt.Fprint(w, "preserved bytes")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "file")
	cmd := command("download", srv.URL, "--output", path, "--quiet")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false

	defer func() {
		if !finished {
			cmd.Process.Kill()
			<-done
		}
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ready := false

	for !ready {
		select {
		case <-ticker.C:
			info, err := os.Stat(path + ".yoinkdl.part")
			ready = err == nil && info.Size() > 0
		case err := <-done:
			finished = true
			t.Fatalf("process stopped before interrupt: %v; %s", err, &errOut)
		case <-deadline.C:
			t.Fatal("process never wrote partial bytes")
		}
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		finished = true
		var exit *exec.ExitError

		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatalf("interrupt result = %v; stderr = %s", err, &errOut)
		}
	case <-deadline.C:
		t.Fatal("process did not stop on interrupt")
	}

	if out.Len() != 0 || !strings.Contains(errOut.String(), "Partial file preserved") {
		t.Fatalf("unexpected output: %q / %q", &out, &errOut)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published interrupted output: %v", err)
	}

	data, err := os.ReadFile(path + ".yoinkdl.part")
	if err != nil || string(data) != "preserved bytes" {
		t.Fatalf("partial = %q, error = %v", data, err)
	}
}

func command(args ...string) *exec.Cmd {
	args = append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "YOINKDL_TEST_PROCESS=1")
	return cmd
}
