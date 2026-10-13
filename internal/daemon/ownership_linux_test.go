package daemon

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"yoinkdl/internal/queue"
)

func TestResolvePaths(t *testing.T) {
	for _, test := range []struct {
		name, home, state, runtime string
		want                       Paths
	}{
		{"defaults", "/home/a", "", "", Paths{"/home/a/.local/state/yoinkdl", "/home/a/.local/state/yoinkdl/run", true}},
		{"xdg", "relative", "/state", "/run/user/1", Paths{"/state/yoinkdl", "/run/user/1/yoinkdl", false}},
		{"relative ignored", "/home/a", "state", "run", Paths{"/home/a/.local/state/yoinkdl", "/home/a/.local/state/yoinkdl/run", true}},
		{"cleaned", "/home/a", "/state/../data/", "/run/user/1/", Paths{"/data/yoinkdl", "/run/user/1/yoinkdl", false}},
		{"invalid runtime", "/home/a", "/state", "/run/\x00bad", Paths{"/state/yoinkdl", "/state/yoinkdl/run", true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolvePaths(test.home, test.state, test.runtime)
			if err != nil || got != test.want {
				t.Fatalf("paths = %+v, %v; want %+v", got, err, test.want)
			}
		})
	}
	for _, path := range []string{"", "relative", "/bad\x00", "/bad\xff"} {
		if _, err := resolvePaths(path, "", ""); !errors.Is(err, ErrPaths) {
			t.Fatal("invalid home accepted")
		}
	}
	if _, err := resolvePaths("/home", "/state", "/"+strings.Repeat("x", 100)); !errors.Is(err, ErrPaths) {
		t.Fatal("overlong socket path accepted")
	}
}

func TestDaemonOwnershipAndPeerIdentity(t *testing.T) {
	p := testPaths(t)
	o := testOwner(t, p)
	check(t, o.listen(probeSocket))
	info := stat(t, p.Socket())
	if info.Mode().Perm() != 0o600 || stat(t, p.Runtime).Mode().Perm() != 0o700 {
		t.Fatal("endpoint is not private")
	}
	lock := stat(t, filepath.Join(p.Runtime, LockName))
	stateLock := stat(t, filepath.Join(p.State, queue.LockName))
	for _, other := range []Paths{p, {State: p.State, Runtime: p.Runtime + "2"}, {State: p.State + "2", Runtime: p.Runtime}} {
		got, err := acquire(other)
		if got != nil || !errors.Is(err, ErrLocked) && !errors.Is(err, queue.ErrLocked) {
			t.Fatalf("competing owner = %v, %v", got, err)
		}
		if !os.SameFile(info, stat(t, p.Socket())) {
			t.Fatal("competing owner replaced the socket")
		}
	}
	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: p.Socket(), Net: "unix"})
	check(t, err)
	defer client.Close()
	server, err := o.listener.AcceptUnix()
	check(t, err)
	defer server.Close()
	for _, conn := range []*net.UnixConn{client, server} {
		check(t, checkPeer(conn, uint32(os.Geteuid())))
		if err := checkPeer(conn, uint32(os.Geteuid())+1); !errors.Is(err, ErrPeer) {
			t.Fatal("mismatching peer UID was accepted")
		}
	}
	check(t, o.close())
	if _, err := os.Lstat(p.Socket()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned socket was not removed")
	}
	if !os.SameFile(lock, stat(t, filepath.Join(p.Runtime, LockName))) ||
		!os.SameFile(stateLock, stat(t, filepath.Join(p.State, queue.LockName))) {
		t.Fatal("persistent lock inode was removed")
	}
	next := testOwner(t, p)
	check(t, next.listen(probeSocket))
}

func TestDaemonStaleSocketAndReplacedCleanup(t *testing.T) {
	p := testPaths(t)
	o := testOwner(t, p)
	old := socketFixture(t, p.Socket())
	check(t, old.Close())
	check(t, o.listen(probeSocket))
	check(t, os.Rename(p.Socket(), p.Socket()+".saved"))
	check(t, os.WriteFile(p.Socket(), []byte("foreign"), 0o640))
	if err := o.close(); !errors.Is(err, ErrChanged) {
		t.Fatalf("replaced cleanup = %v", err)
	}
	data, err := os.ReadFile(p.Socket())
	check(t, err)
	if string(data) != "foreign" || stat(t, p.Socket()).Mode().Perm() != 0o640 {
		t.Fatal("cleanup changed a replacement artifact")
	}
}

func TestDaemonPreservesUnownedEndpoints(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink", "live", "public socket", "timeout", "permission", "changed"} {
		t.Run(kind, func(t *testing.T) {
			p := testPaths(t)
			o := testOwner(t, p)
			probe := probeSocket
			switch kind {
			case "file":
				check(t, os.WriteFile(p.Socket(), []byte("keep"), 0o600))
			case "directory":
				check(t, os.Mkdir(p.Socket(), 0o700))
			case "symlink":
				check(t, os.Symlink(filepath.Join(p.State, queue.SnapshotName), p.Socket()))
			default:
				listener := socketFixture(t, p.Socket())
				if kind != "live" {
					check(t, listener.Close())
				}
				if kind == "public socket" {
					check(t, os.Chmod(p.Socket(), 0o666))
				}
			}
			before := stat(t, p.Socket())
			if kind == "timeout" {
				probe = func(string) error { return syscall.ETIMEDOUT }
			} else if kind == "permission" {
				probe = func(string) error { return syscall.EACCES }
			} else if kind == "changed" {
				probe = func(string) error {
					check(t, os.Remove(p.Socket()))
					check(t, os.WriteFile(p.Socket(), []byte("replacement"), 0o600))
					before = stat(t, p.Socket())
					return syscall.ECONNREFUSED
				}
			}
			if err := o.listen(probe); err == nil {
				t.Fatal("unsafe endpoint was replaced")
			}
			check(t, o.close())
			after := stat(t, p.Socket())
			if !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Fatal("refusal changed the existing endpoint")
			}
		})
	}
}

func TestDaemonRefusesUnsafeDirectoriesAndLocks(t *testing.T) {
	for _, kind := range []string{"base mode", "base symlink", "runtime mode", "runtime symlink", "lock mode", "lock symlink", "lock link", "lock fifo"} {
		t.Run(kind, func(t *testing.T) {
			p := testPaths(t)
			check(t, os.Mkdir(p.Runtime, 0o700))
			lock := filepath.Join(p.Runtime, LockName)
			switch kind {
			case "base mode":
				check(t, os.Chmod(filepath.Dir(p.Runtime), 0o755))
			case "base symlink":
				link := filepath.Join(filepath.Dir(p.Runtime), "alias")
				check(t, os.Symlink(filepath.Dir(p.Runtime), link))
				p.Runtime = filepath.Join(link, "run")
			case "runtime mode":
				check(t, os.Chmod(p.Runtime, 0o755))
			case "runtime symlink":
				check(t, os.Rename(p.Runtime, p.Runtime+".saved"))
				check(t, os.Symlink(p.Runtime+".saved", p.Runtime))
			case "lock mode":
				check(t, os.WriteFile(lock, nil, 0o644))
			case "lock symlink":
				check(t, os.Symlink("foreign", lock))
			case "lock link":
				check(t, os.WriteFile(lock, nil, 0o600))
				check(t, os.Link(lock, lock+".alias"))
			case "lock fifo":
				check(t, syscall.Mkfifo(lock, 0o600))
			}
			if o, err := acquire(p); o != nil || err == nil {
				if o != nil {
					o.close()
				}
				t.Fatalf("unsafe %s accepted", kind)
			}
		})
	}
}

func TestDaemonFallbackCreatesMissingStateParents(t *testing.T) {
	p := testPaths(t)
	p.State = filepath.Join(filepath.Dir(p.State), ".local", "state", "yoinkdl")
	p.Runtime, p.Fallback = filepath.Join(p.State, "run"), true
	o := testOwner(t, p)
	check(t, o.listen(probeSocket))
	for _, path := range []string{p.State, p.Runtime, filepath.Dir(p.State)} {
		if stat(t, path).Mode().Perm() != 0o700 {
			t.Fatal("fallback directory is not private")
		}
	}
}

func TestDaemonRefusesChangedOwnership(t *testing.T) {
	for _, kind := range []string{"directory", "lock"} {
		t.Run(kind, func(t *testing.T) {
			p := testPaths(t)
			o := testOwner(t, p)
			if kind == "directory" {
				check(t, os.Rename(p.Runtime, p.Runtime+".saved"))
				check(t, os.Mkdir(p.Runtime, 0o700))
			} else {
				path := filepath.Join(p.Runtime, LockName)
				check(t, os.Rename(path, path+".saved"))
				check(t, os.WriteFile(path, nil, 0o600))
			}
			if err := o.listen(probeSocket); !errors.Is(err, ErrChanged) {
				t.Fatalf("changed %s accepted: %v", kind, err)
			}
			if _, err := os.Lstat(p.Socket()); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("changed ownership created a socket")
			}
		})
	}
}

func TestDaemonRejectsForeignArtifactOwners(t *testing.T) {
	p := testPaths(t)
	o := testOwner(t, p)
	check(t, o.listen(probeSocket))
	for _, path := range []string{p.Runtime, filepath.Join(p.Runtime, LockName), p.Socket()} {
		info := stat(t, path)
		foreign := *info.Sys().(*syscall.Stat_t)
		foreign.Uid++
		info = foreignInfo{FileInfo: info, stat: &foreign}
		if privateDirectory(info) || privateLock(info) || ownedSocket(info) {
			t.Fatal("foreign artifact owner accepted")
		}
	}
}

type foreignInfo struct {
	os.FileInfo
	stat *syscall.Stat_t
}

func (f foreignInfo) Sys() any {
	return f.stat
}

func testPaths(t *testing.T) Paths {
	t.Helper()
	dir, err := os.MkdirTemp("", "ydl-ipc-")
	check(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return Paths{State: filepath.Join(dir, "state"), Runtime: filepath.Join(dir, "run")}
}

func testOwner(t *testing.T, p Paths) *owner {
	t.Helper()
	o, err := acquire(p)
	check(t, err)
	t.Cleanup(func() { check(t, o.close()) })
	return o
}

func socketFixture(t *testing.T, path string) *net.UnixListener {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	check(t, err)
	listener.SetUnlinkOnClose(false)
	t.Cleanup(func() { listener.Close() })
	check(t, os.Chmod(path, 0o600))
	return listener
}

func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	check(t, err)
	return info
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
