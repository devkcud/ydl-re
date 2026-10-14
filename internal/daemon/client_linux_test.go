package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yoinkdl/internal/queue"
)

func TestDaemonClientNeverStartsOrCreatesState(t *testing.T) {
	p := testPaths(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Call(ctx, p, "ping", map[string]any{}); !errors.Is(err, ErrConnection) {
		t.Fatalf("missing daemon = %v", err)
	}
	for _, path := range []string{p.State, p.Runtime} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("client created daemon artifacts")
		}
	}
}

func TestDaemonClientLostReplyIsUncertainOnlyForMutations(t *testing.T) {
	for _, op := range []string{"list", "set_parallel"} {
		t.Run(op, func(t *testing.T) {
			p := testPaths(t)
			o := testOwner(t, p)
			check(t, o.listen(probeSocket))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				args := map[string]any{}
				if op == "set_parallel" {
					args["parallel"] = 2
				}
				_, err := Call(ctx, p, op, args)
				done <- err
			}()
			check(t, o.listener.SetDeadline(time.Now().Add(time.Second)))
			conn, err := o.listener.AcceptUnix()
			check(t, err)
			defer conn.Close()
			check(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
			data, err := readFrame(conn, MaxRequestSize)
			check(t, err)
			if _, err := decodeCommand(data, p.State); err != nil {
				t.Fatal(err)
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || errors.Is(err, queue.ErrOutcomeUnknown) != (op == "set_parallel") {
					t.Fatalf("lost %s reply = %v", op, err)
				}
			case <-time.After(time.Second):
				t.Fatal("client cancellation did not interrupt the read")
			}
		})
	}
}

func TestDaemonClientRejectsUnsafeOrMalformedPeer(t *testing.T) {
	for _, kind := range []string{"public root", "public socket", "socket symlink", "bad reply"} {
		t.Run(kind, func(t *testing.T) {
			p := testPaths(t)
			o := testOwner(t, p)
			check(t, o.listen(probeSocket))
			switch kind {
			case "public root":
				check(t, os.Chmod(p.Runtime, 0o755))
			case "public socket":
				check(t, os.Chmod(p.Socket(), 0o666))
			case "socket symlink":
				check(t, os.Rename(p.Socket(), p.Socket()+".saved"))
				check(t, os.Symlink(SocketName+".saved", p.Socket()))
				t.Cleanup(func() {
					check(t, os.Remove(p.Socket()))
					check(t, os.Rename(p.Socket()+".saved", p.Socket()))
				})
			case "bad reply":
				go func() {
					conn, err := o.listener.AcceptUnix()
					if err != nil {
						return
					}
					defer conn.Close()
					conn.SetDeadline(time.Now().Add(time.Second))
					readFrame(conn, MaxRequestSize)
					writeFrame(conn, []byte("{\"secret\":\"private\"}\n"))
				}()
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := Call(ctx, p, "set_parallel", map[string]any{"parallel": 2})
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe peer error = %v", err)
			}
			if kind == "bad reply" && (!errors.Is(err, ErrProtocol) || !errors.Is(err, queue.ErrOutcomeUnknown)) {
				t.Fatal("malformed mutation reply lost uncertain outcome")
			}
		})
	}
}

func TestDaemonStateMismatchAndMalformedFramesDoNotMutate(t *testing.T) {
	p := testPaths(t)
	_, done := startServer(t, Options{Paths: p}, nil)
	wrong := p
	wrong.State = filepath.Join(filepath.Dir(p.State), "other")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := Call(ctx, wrong, "set_parallel", map[string]any{"parallel": 2})
	if err == nil || r.Error == nil || r.Error.Code != "state_mismatch" {
		t.Fatalf("state mismatch = %+v, %v", r, err)
	}
	for _, data := range []string{
		"{\"secret\":\"private\"}\n", strings.Repeat("x", MaxRequestSize+1) + "\n",
		string(requestFrame(t, p.State, "ping", map[string]any{"url": "private"})),
	} {
		conn := rawClient(t, p)
		writeFrame(conn, []byte(data))
		reply, err := readFrame(conn, MaxReplySize)
		check(t, err)
		conn.Close()
		if strings.Contains(string(reply), "private") {
			t.Fatal("malformed request data reached the reply")
		}
		r, err := decodeReply(reply, "ping")
		if err != nil || r.OK || r.Error.Code != "invalid_request" {
			t.Fatalf("invalid frame response = %+v, %v", r, err)
		}
	}
	waitList(t, p, func(v List) bool {
		if v.Daemon.Parallel != 1 || len(v.Jobs) != 0 {
			t.Fatal("invalid request mutated the queue")
		}
		return true
	})
	call(t, p, "stop", map[string]any{})
	check(t, waitServer(t, done))
}
