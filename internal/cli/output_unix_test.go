//go:build linux || darwin

package cli

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRecordWriterOwnsOnlyItsDuplicate(t *testing.T) {
	reader, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer out.Close()
	fd := out.Fd()
	w, err := newRecordWriter(out)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if w.out == out || w.deadline == nil {
		t.Fatal("pipe output did not get an owned cancellable handle")
	}
	if n, err := w.Write(context.Background(), []byte("a")); n != 1 || err != nil {
		t.Fatalf("write = %d, %v", n, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 || flags&syscall.O_NONBLOCK != 0 {
		t.Fatalf("original descriptor flags changed: flags=%d, error=%v", flags, errno)
	}
	if _, err := out.Write([]byte("b")); err != nil {
		t.Fatalf("original output was closed: %v", err)
	}
	buf := make([]byte, 2)
	if n, err := reader.Read(buf); err != nil || n != 2 || string(buf) != "ab" {
		t.Fatalf("pipe data = %q, %v", buf, err)
	}
}

func TestRecordWriterCancelsFullPipe(t *testing.T) {
	reader, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer out.Close()
	w, err := newRecordWriter(out)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	raw, err := out.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Control(func(fd uintptr) {
		buf := make([]byte, 65536)
		for {
			_, err := syscall.Write(int(fd), buf)
			if errors.Is(err, syscall.EAGAIN) {
				return
			}
			if err != nil {
				t.Error(err)
				return
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := w.Write(ctx, []byte("record\n"))
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled pipe = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pipe write did not honor cancellation")
	}
}
