# Platform qualification

[Documentation index](README.md)

The native evidence covers Linux/amd64. The local results were recorded on October 12, 2026, against runtime and test sources at `ee2f29558b3b162f4ad39fc6682ba7edf878f240`. The later [hosted CI run](#hosted-ci-evidence) covers packaging tests at `1ce24810b7b6a05607c30ab37ff725758e29ebb8`; [stable publication evidence](releases.md#stable-publication) records `0.1.0` at `31330c6b07d33af148341bdfb346c0e139082fdb`. Other native platforms and power-loss behavior remain unqualified.

## Build and runtime matrix

| Target | CLI build | Test programs | Native runtime evidence | Current status |
|---|---|---|---|---|
| `linux/amd64` | Pass | Compiled and run | Local ext4/tmpfs checks; hosted Ubuntu 24.04.5 gate, race, and CGO-disabled tests | Qualified initial stable binary target |
| `linux/arm64` | Pass | Compile only | Unavailable on this host | Storage enabled; native behavior unqualified |
| `darwin/amd64` | Pass | Compile only | Unavailable on this host | Storage enabled; native behavior unqualified |
| `darwin/arm64` | Pass | Compile only | Unavailable on this host | Storage enabled; native behavior unqualified |
| `windows/amd64` | Pass | Compile only | Unavailable on this host | Download/inspection storage unsupported by design |
| `windows/arm64` | Pass | Compile only | Unavailable on this host | Download/inspection storage unsupported by design |

All six local matrix CLI builds used `CGO_ENABLED=0`, `-trimpath`, and the default `dev` label. `file` identified the intended ELF, Mach-O, and PE architectures. The original local run compiled five Go test packages for each foreign target; the hosted run also includes the later `tools` package. No foreign binary or test program was executed. The separate native full gate used CGO for race testing. No cross-C compiler was needed.

Linux/amd64 is the only platform with native qualification in this report. The code enables Linux and macOS storage; compilation alone does not qualify another architecture, locking, signals, terminal handling, filesystem semantics, or an OS version. Windows builds retain explicit storage refusal and are not usable downloader releases. Other targets have not been checked. The temporary build outputs are validation artifacts, with no installer, signing, packaging, or distribution contract.

## Internal v0.2 daemon runtime

E4's internal daemon API is Linux-only and requires `/proc/self/fd` for anchored socket operations. Non-Linux construction, path resolution, and client calls return `ErrUnsupported`. Linux/amd64 ordinary/race and CGO-disabled daemon tests passed on Go 1.27.2. The six-target CLI/test compilation recipe passed with the new package; final Linux shutdown changes also passed native tests and Linux/arm64 daemon test compilation. Foreign test programs were not executed. E5 adds ordinary/race/CGO-disabled CLI and real-process tests, plus six-target compilation, for the development commands.

E6 forced-death, checkpoint/FIFO recovery, pending-control, publication-boundary, and retained-capacity/forget fixtures pass on Linux/amd64 with ordinary/race and focused CGO-disabled tests. The [testing report](testing.md#current-evidence) distinguishes actual CLI processes from the private runner seam used to hold results at crash boundaries. Broader process concurrency/rate, whole-suite CGO-disabled, and six-target gates remain pending; these checks do not change published stable binary support.

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

## Hosted CI evidence

[CI run 38294238535](https://github.com/devkcud/ydl-re/actions/runs/38294238535) passed on October 12, 2026, for main-branch commit `1ce24810b7b6a05607c30ab37ff725758e29ebb8`. Both jobs completed successfully; their logs confirm these values:

| Item | Observed value |
|---|---|
| Runner | Ubuntu 24.04.5 LTS, image `ubuntu-24.04` version `20261004.327.1` |
| Go | `go1.27.0 linux/amd64`, selected from `go.mod` |
| Toolchain settings | `GOTOOLCHAIN=local`, `GOAMD64=v1`, no `GOEXPERIMENT` |
| Native gate | `make check`: formatting, vet, tests, race tests, and build passed with CGO enabled |
| CGO-disabled runtime | `CGO_ENABLED=0 go test -count=1 ./...` passed for all six packages, including packaging tests |
| Compilation | CGO-disabled CLI and test compilation passed for all six matrix targets |

This verifies the project's minimum Go patch on the hosted runner. The job does not record filesystem or mount details, so it adds no filesystem-specific or power-loss guarantee. Packaging tests build and exercise temporary fixture archives; this CI run did not execute the tag-triggered draft workflow or upload release assets.

## Reproduce native checks

Run from the repository root with Go 1.27 or newer, GNU Make, a working C compiler for race tests, and permission to use localhost listeners. Record the actual toolchain and filesystem rather than assuming the defaults match this report.

```sh
set -eu
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

## Reproduce compilation checks

Run this POSIX shell block from the repository root. It creates separate temporary CLI/test outputs per target and stops on the first failure. `go test -c` compiles without claiming a test run; do not substitute an execution wrapper that reports foreign tests as passing.

```sh
set -eu
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/yoinkdl-platform.XXXXXX")
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
    os=${target%/*}
    arch=${target#*/}
    out="$build_dir/$os-$arch"
    ext=
    if [ "$os" = windows ]; then
        ext=.exe
    fi
    mkdir -p "$out/tests"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -p 2 -trimpath -o "$out/yoinkdl$ext" ./cmd/yoinkdl
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go test -p 2 -c -o "$out/tests/" ./...
done
printf '%s\n' "$build_dir"
```

Use `go version -m` and `file` on the generated CLI files to inspect the recorded source revision, CGO setting, and target format. Repeat compilation whenever platform-specific code or build tags change. Native checks are still required before expanding the qualified runtime matrix.

## Qualification limits

These are ordinary filesystem and forced-process-termination results on the local host and hosted CI runner. They do not simulate loss of power, a failing disk/controller, full disks, network filesystems, or all mount configurations. Directory sync and atomic visibility still need qualification wherever storage support is claimed. Broader sync/cleanup fault injection remains open.

Native Linux/arm64 and macOS execution have not been performed here. Windows storage remains deliberately unsupported. The [CI/release tooling](releases.md) selects Linux/amd64 for initial binary releases. Hosted CI, release packaging, and [stable public downloads and installation](releases.md#stable-publication) passed for `0.1.0`. The [decision register](decisions.md#release-and-compatibility-decisions) tracks remaining qualification.
