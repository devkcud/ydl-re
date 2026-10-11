package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

func TestRecordWriterDeadlineAndCancellation(t *testing.T) {
	for _, cancelWrite := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			out, reader := net.Pipe()
			defer out.Close()
			defer reader.Close()
			w, err := newRecordWriter(out)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			start := time.Now()
			go func() {
				n, err := w.Write(ctx, []byte("record\n"))
				if n != 0 {
					t.Error("blocked output delivered bytes")
				}
				done <- err
			}()
			synctest.Wait()
			want := os.ErrDeadlineExceeded
			elapsed := outputTimeout
			if cancelWrite {
				want = context.Canceled
				elapsed = 0
				cancel()
			}
			if err := <-done; !errors.Is(err, want) || time.Since(start) != elapsed {
				t.Fatalf("blocked write = %v, elapsed=%s", err, time.Since(start))
			}
			read := make(chan string, 1)
			go func() {
				data := make([]byte, 3)
				if _, err := io.ReadFull(reader, data); err != nil {
					t.Error(err)
				}
				read <- string(data)
			}()
			if n, err := w.Write(context.Background(), []byte("ok\n")); n != 3 || err != nil || <-read != "ok\n" {
				t.Fatalf("previous deadline poisoned a later write: %d, %v", n, err)
			}
		})
	}
}
