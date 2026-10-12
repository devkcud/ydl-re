# Platform qualification

[Documentation index](README.md)

The current native evidence is for Linux/amd64 development builds. It does not establish a v0.1 release, other native platforms, or power-loss guarantees. The results below were recorded on October 12, 2026, against runtime and test sources at `ee2f29558b3b162f4ad39fc6682ba7edf878f240`.

## Native Linux evidence

| Item | Observed value |
|---|---|
| Host | Arch Linux, `linux/amd64`, kernel `7.2.9-zen1-1-zen` |
| Go | `go1.27.2-X:nodwarf5`; `GOEXPERIMENT=nodwarf5` |
| CPU build baseline | `GOAMD64=v1` |
| Race environment | `CGO_ENABLED=1`, GCC `16.2.1 20260810`, unprivileged UID 1000 |
| Workspace filesystem | Local ext4 |
| Default temporary filesystem | tmpfs at `/tmp` |
| Source tree when checked | Clean; no product-code changes required |

`make check` passed with `TMPDIR` set to a private directory under the workspace's ignored `bin/` directory. This places the filesystem, HTTP, CLI, and subprocess fixtures on ext4 instead of the default tmpfs. Formatting, vet, ordinary tests, race tests, and build passed. The pure range-planning package reused its matching cached result; the packages exercising files and processes ran with the new temporary directory.

`CGO_ENABLED=0 go test -count=1 ./...` also passed on Linux/amd64 with the default tmpfs temporary directory. This run was uncached and did not use the race detector. The separate CGO-disabled CLI build was identified as a statically linked ELF executable, and its `version` command returned `yoinkdl dev`. This does not assert equivalent DNS behavior between Go's native and cgo resolvers.

The existing suites exercise prefix/chunk resume, forced process termination, stable advisory locks, hard-link and rename recovery, private partial permissions, symlink/collision refusal, stored-byte verification, cancellation, SIGINT/SIGTERM, terminal geometry, and blocked/broken output. See [testing coverage](testing.md#current-evidence).

## Reproduce native checks

Run from the repository root with Go 1.27 or newer, GNU Make, a working C compiler for race tests, and permission to use localhost listeners. Record the actual toolchain and filesystem rather than assuming the defaults match this report.

```sh
go version
go env GOOS GOARCH GOAMD64 CGO_ENABLED CC GOEXPERIMENT
uname -srm
findmnt --noheadings --target . --output TARGET,FSTYPE
findmnt --noheadings --target /tmp --output TARGET,FSTYPE
mkdir -p bin
check_tmp=$(mktemp -d "$PWD/bin/platform.XXXXXX")
TMPDIR="$check_tmp" make check
rmdir "$check_tmp"
CGO_ENABLED=0 go test -count=1 ./...
```

`findmnt` is a Linux inspection command. Confirm the private temporary directory is on the filesystem being qualified. `rmdir` removes only an empty directory; investigate retained fixtures if a failed run leaves it nonempty. Tests need no public service and use deterministic local HTTP fixtures.

## Qualification limits

These are ordinary filesystem and forced-process-termination results on one host. They do not simulate loss of power, a failing disk/controller, full disks, network filesystems, or all mount configurations. Directory sync and atomic visibility still need qualification wherever storage support is claimed. Broader sync/cleanup fault injection remains open.

Native Linux/arm64 and macOS execution have not been performed here. Windows storage remains deliberately unsupported. Native CI coverage and a published support policy remain release work; the [decision register](decisions.md#release-and-compatibility-decisions) tracks those requirements.
