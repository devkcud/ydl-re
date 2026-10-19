# Platform qualification

[Documentation index](README.md)

The latest local qualification covers the completed E6 runtime/test sources at clean revision `5c00f2e1e522268c27ab89aba7d1f479fa12950f`, checked on October 19, 2026. Native Linux/amd64 ordinary/race/CGO-disabled gates and six-target compilation passed. The [local 0.2.0-rc.1 candidate](releases.md#v02-local-qualification) also passed packaging and installed-binary checks at `34041c372519ac751bb9d6ec1ecda56184d13161`, with unchanged runtime/test sources. No hosted v0.2 run or publication is claimed.

The earlier October 12 native report covered `ee2f29558b3b162f4ad39fc6682ba7edf878f240`. The [hosted CI run](#hosted-ci-evidence) and [stable publication evidence](releases.md#stable-publication) below remain historical v0.1 evidence. Other native platforms and power-loss behavior remain unqualified.

## Build and runtime matrix

| Target | CLI build | Test programs | Native runtime evidence | Current status |
|---|---|---|---|---|
| `linux/amd64` | Pass | Compiled and run | E6 ext4 ordinary/race gate and tmpfs CGO-disabled suite; historical v0.1 hosted checks | Locally qualified v0.2 runtime; stable `0.1.0` binary target |
| `linux/arm64` | Pass | Compile only | Unavailable on this host | Storage enabled; native behavior unqualified |
| `darwin/amd64` | Pass | Compile only | Unavailable on this host | Storage enabled; native behavior unqualified |
| `darwin/arm64` | Pass | Compile only | Unavailable on this host | Storage enabled; native behavior unqualified |
| `windows/amd64` | Pass | Compile only | Unavailable on this host | Download/inspection storage unsupported by design |
| `windows/arm64` | Pass | Compile only | Unavailable on this host | Download/inspection storage unsupported by design |

All six E6 matrix CLI builds used `CGO_ENABLED=0`, `-trimpath`, and the default `dev` label. Each target compiled all eight test packages, including queue, daemon, and packaging tests. `file` identified the intended ELF, Mach-O, and PE architectures; `go version -m` recorded the checked revision and `vcs.modified=false`. Only the Linux/amd64 CLI's version command was executed from the matrix outputs; native test execution came from the separate full suites. No foreign binary or test program was executed. The native full gate used CGO for race testing. No cross-C compiler was needed.

Linux/amd64 is the only platform with native qualification in this report. The code enables Linux and macOS storage; compilation alone does not qualify another architecture, locking, signals, terminal handling, filesystem semantics, or an OS version. Windows builds retain explicit storage refusal and are not usable downloader releases. Other targets have not been checked. The temporary build outputs are validation artifacts, with no installer, signing, packaging, or distribution contract.

## Internal v0.2 daemon runtime

E4's internal daemon API is Linux-only and requires `/proc/self/fd` for anchored socket operations. Non-Linux construction, path resolution, and client calls return `ErrUnsupported`. Linux/amd64 ordinary/race and CGO-disabled daemon tests passed on Go 1.27.2. The six-target CLI/test compilation recipe passed with the new package; final Linux shutdown changes also passed native tests and Linux/arm64 daemon test compilation. Foreign test programs were not executed. E5 adds ordinary/race/CGO-disabled CLI and real-process tests, plus six-target compilation, for the development commands.

E6 forced-death, checkpoint/FIFO recovery, pending-control, publication-boundary, capacity/forget, live concurrency, and shared-rate fixtures passed the final Linux/amd64 gate and uncached whole-suite CGO-disabled run. Six-target compilation and local candidate archive/installation verification also passed. The [testing report](testing.md#current-evidence) distinguishes actual CLI processes from private runner seams used to hold results at precise boundaries. These checks do not change published stable binary support.

## Native Linux evidence

| Item | Observed value |
|---|---|
| Host | Arch Linux, `linux/amd64`, kernel `7.2.11-zen1-1-zen` |
| Go | `go1.27.2-X:nodwarf5`; `GOEXPERIMENT=nodwarf5` |
| CPU build baseline | `GOAMD64=v1` |
| Race environment | `CGO_ENABLED=1`, GCC `16.2.1 20260810`, unprivileged UID 1000 |
| Workspace filesystem | Local ext4 |
| Default temporary filesystem | tmpfs at `/tmp` |
| Source tree when checked | Clean; no product-code changes required |

`TMPDIR="$PWD/bin/t.MBOfTk" make check` passed with the private temporary directory on ext4. Formatting, vet, ordinary tests, race tests, and build passed. All seven packages exercising files/processes reran in both ordinary and race modes; only pure range planning reused matching cached results. The CLI/process suite took 18.472 seconds ordinarily and 75.604 seconds with the race detector.

`CGO_ENABLED=0 go test -count=1 ./...` also passed on Linux/amd64 with the default tmpfs temporary directory. This run was uncached and did not use the race detector. The separate CGO-disabled CLI build was identified as a statically linked ELF executable, and its `version` command returned `yoinkdl dev`. This does not assert equivalent DNS behavior between Go's native and cgo resolvers.

The existing suites exercise prefix/chunk resume, forced process termination, stable advisory locks, hard-link and rename recovery, private partial permissions, symlink/collision refusal, stored-byte verification, cancellation, SIGINT/SIGTERM, terminal geometry, and blocked/broken output. See [testing coverage](testing.md#current-evidence).

Local logs and the eight-package-per-target outputs are retained under ignored `bin/e6-qualification.0Stkvu/`: `native-ext4.log`, `cgo-disabled.log`, `matrix-host.log`, and `environment.json`. The first sandbox attempt could not open local listeners. A longer ext4 temporary path also correctly triggered the daemon's socket-length refusal; shortening the path fixed the fixture environment. Sandboxed cross-compilation reported unresolved macOS standard-library packages despite their files being present; the identical six-target recipe passed outside the sandbox. These failed attempts remain in the same evidence directory. No product changes were required.

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
check_tmp=$(mktemp -d "$PWD/bin/t.XXXXXX")
TMPDIR="$check_tmp" make check
rmdir "$check_tmp"
CGO_ENABLED=0 go test -count=1 ./...
```

`findmnt` is a Linux inspection command. Confirm the private temporary directory is on the filesystem being qualified. Keep its absolute path short: daemon fixtures append state/runtime directories and `control.sock`, whose full Linux pathname must be shorter than 108 bytes. If the checkout path is too long, select a shorter private directory on the same filesystem. `rmdir` removes only an empty directory; investigate retained fixtures if a failed run leaves it nonempty. Tests need no public service and use deterministic local HTTP fixtures.

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
