package queue

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStoreCompetingOpen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	owners := make(chan *Store, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			s, err := Open(dir)
			if err == nil {
				owners <- s
			} else if !errors.Is(err, ErrLocked) {
				t.Errorf("unexpected contention error: %v", err)
			}
		})
	}
	wg.Wait()
	close(owners)
	count := 0
	for s := range owners {
		count++
		must(t, s.Close())
	}
	if count != 1 {
		t.Fatalf("got %d writers", count)
	}
	assertStore(t, testStore(t, dir), NewSnapshot())
}

func TestStoreLockSurvivesUntilProcessDeath(t *testing.T) {
	dir := privateDir(t)
	s := testStore(t, dir)
	want := testSnapshot(t, dir, 1)
	want.Jobs[0].State = Running
	want.Jobs[0].Published = nil
	must(t, s.Save(want))
	must(t, s.Close())
	lock, err := os.Lstat(filepath.Join(dir, LockName))
	must(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStoreProcessHelper$")
	cmd.Env = append(os.Environ(), "YOINKDL_QUEUE_LOCK_TEST="+dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	must(t, err)
	in, err := cmd.StdinPipe()
	must(t, err)
	defer in.Close()
	must(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	ready, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || ready != "ready\n" {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("helper did not acquire ownership: %v, %s", err, stderr.String())
	}
	other, err := Open(dir)
	if other != nil || !errors.Is(err, ErrLocked) {
		t.Fatal("second process acquired the live store")
	}
	must(t, cmd.Process.Kill())
	if err := cmd.Wait(); err == nil {
		t.Fatal("helper was not forcibly terminated")
	}

	s = testStore(t, dir)
	assertStore(t, s, want)
	after, err := os.Lstat(filepath.Join(dir, LockName))
	if err != nil || !os.SameFile(lock, after) {
		t.Fatal("persistent lock inode was removed or replaced")
	}
}

func TestStoreProcessHelper(t *testing.T) {
	dir := os.Getenv("YOINKDL_QUEUE_LOCK_TEST")
	if dir == "" {
		return
	}
	s, err := Open(dir)
	must(t, err)
	defer s.Close()
	fmt.Fprintln(os.Stdout, "ready")
	io.Copy(io.Discard, os.Stdin)
}
