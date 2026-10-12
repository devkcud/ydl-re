package tools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReleaseArtifacts(t *testing.T) {
	dir := releaseFixture(t)
	const version = "0.1.0-rc.1"
	run(t, dir, "bash", "tools/release.sh", version)
	out := filepath.Join(dir, "bin/releases", version)
	sums := read(t, filepath.Join(out, "SHA256SUMS"))
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if len(lines) != 2 {
		t.Fatalf("checksum count = %d", len(lines))
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 || filepath.Base(fields[1]) != fields[1] {
			t.Fatalf("invalid checksum line: %q", line)
		}
		data := read(t, filepath.Join(out, fields[1]))
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatal("archive checksum mismatch")
		}
	}

	name := "yoinkdl_" + version + "_linux_amd64"
	source := "yoinkdl_" + version + "_source"
	pkg := archive(t, filepath.Join(out, name+".tar.gz"))
	src := archive(t, filepath.Join(out, source+".tar.gz"))
	for _, file := range []string{"LICENSE", "SECURITY.md", "CHANGELOG.md"} {
		want := read(t, filepath.Join(dir, file))
		if !bytes.Equal(pkg[name+"/"+file], want) || !bytes.Equal(src[source+"/"+file], want) {
			t.Fatalf("missing or mismatched %s", file)
		}
	}
	if !bytes.Equal(src[source+"/cmd/yoinkdl/main.go"], read(t, filepath.Join(dir, "cmd/yoinkdl/main.go"))) {
		t.Fatal("source archive differs from the built tree")
	}
	rev := strings.TrimSpace(string(run(t, dir, "git", "rev-parse", "HEAD")))
	if !bytes.Contains(pkg[name+"/SOURCE.txt"], []byte(rev)) || len(pkg[name+"/LICENSE-Go"]) == 0 {
		t.Fatal("missing source identity or runtime license")
	}

	bin := filepath.Join(t.TempDir(), "yoinkdl")
	if err := os.WriteFile(bin, pkg[name+"/yoinkdl"], 0o700); err != nil {
		t.Fatal(err)
	}
	if got := string(run(t, dir, bin, "version")); got != "yoinkdl "+version+"\n" {
		t.Fatalf("packaged version = %q", got)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "packaged transfer bytes")
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	run(t, dir, bin, "download", srv.URL, "-o", path, "--quiet")
	if string(read(t, path)) != "packaged transfer bytes" {
		t.Fatal("packaged executable downloaded different bytes")
	}

	if _, err := command(t, dir, "bash", "tools/release.sh", version).CombinedOutput(); err == nil {
		t.Fatal("replaced existing release output")
	}
	if !bytes.Equal(sums, read(t, filepath.Join(out, "SHA256SUMS"))) {
		t.Fatal("refused package changed existing checksums")
	}
	if err := os.Rename(out, out+".saved"); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "bash", "tools/release.sh", version)
	if !bytes.Equal(sums, read(t, filepath.Join(out, "SHA256SUMS"))) {
		t.Fatal("same source and toolchain produced different archives")
	}
}

func TestReleaseRefusesInvalidState(t *testing.T) {
	dir := releaseFixture(t)
	for _, version := range []string{"", "dev", "v0.1.0", "01.1.0", "0.1.0;touch injected", "../other", "0.1.0-rc.0"} {
		if _, err := command(t, dir, "bash", "tools/release.sh", version).CombinedOutput(); err == nil {
			t.Fatalf("accepted invalid version %q", version)
		}
	}

	path := filepath.Join(dir, "uncommitted")
	if err := os.WriteFile(path, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, staged := range []bool{false, true} {
		if staged {
			run(t, dir, "git", "add", "uncommitted")
		}
		if _, err := command(t, dir, "bash", "tools/release.sh", "0.1.0").CombinedOutput(); err == nil {
			t.Fatal("packaged an uncommitted tree")
		}
	}
	if string(read(t, path)) != "preserve" {
		t.Fatal("changed refused state")
	}
	if _, err := os.Stat(filepath.Join(dir, "bin/releases")); !os.IsNotExist(err) {
		t.Fatalf("refused input created artifacts: %v", err)
	}
}

func TestReleaseFailurePreservesOutputs(t *testing.T) {
	for name, source := range map[string]string{
		"build": "package main\ninvalid Go\n",
		"version exit": `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("yoinkdl 0.1.0")
		os.Exit(7)
	}
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := releaseFixture(t)
			path := filepath.Join(dir, "cmd/yoinkdl/main.go")
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			run(t, dir, "git", "add", "cmd/yoinkdl/main.go")
			run(t, dir, "git", "-c", "user.name=Release test", "-c", "user.email=test@example.invalid",
				"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "broken executable")
			base := filepath.Join(dir, "bin/releases")
			if err := os.MkdirAll(base, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(base, "keep"), []byte("previous artifact"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := command(t, dir, "bash", "tools/release.sh", "0.1.0").CombinedOutput(); err == nil {
				t.Fatal("packaged a failed executable")
			}
			entries, err := os.ReadDir(base)
			if err != nil || len(entries) != 1 || entries[0].Name() != "keep" {
				t.Fatalf("failure left partial artifacts: %v, %v", entries, err)
			}
			if string(read(t, filepath.Join(base, "keep"))) != "previous artifact" {
				t.Fatal("failure changed previous artifacts")
			}
		})
	}
}

func releaseFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("release packaging requires native Linux/amd64 and GNU tools")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "checkout with spaces")
	files := run(t, root, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	for _, name := range bytes.Split(files, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		path := filepath.Join(dir, string(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, read(t, filepath.Join(root, string(name))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, dir, "git", "init", "--quiet")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.name=Release test", "-c", "user.email=test@example.invalid",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "fixture")
	return dir
}

func command(t *testing.T, dir, name string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	return cmd
}

func run(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	cmd := command(t, dir, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func archive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(read(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	files := make(map[string][]byte)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			files[header.Name], err = io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
