# CI and releases

[Documentation index](README.md)

Initial binary releases target Linux/amd64 through [GitHub Releases](https://github.com/devkcud/ydl-re/releases). Project sources use [MPL 2.0](../LICENSE); the binary also carries the Go runtime's [BSD notice](../LICENSES/Go-BSD-3-Clause.txt). Security reports use public [GitHub Issues](../SECURITY.md). No stable version has been published by this work.

## Continuous integration

[CI](../.github/workflows/ci.yml) runs on pull requests, main-branch pushes, and manual dispatch. Ubuntu 24.04 runs the existing `make check` gate and an uncached CGO-disabled test run. A separate job cross-builds the CLI and test programs for Linux, macOS, and Windows on amd64/arm64. Foreign test programs are compiled only; Windows storage remains unsupported.

Both workflows pin [checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1) and [setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0) to full commit IDs. The Go version comes from `go.mod`; dependency caching is disabled because the project has no external Go modules or `go.sum`. Checkout does not persist credentials. CI has read-only repository permissions, time limits, and cancellation of superseded runs.

[CI run 38294238535](https://github.com/devkcud/ydl-re/actions/runs/38294238535) passed on October 12, 2026, for commit `1ce24810b7b6a05607c30ab37ff725758e29ebb8`. Both jobs used Ubuntu 24.04.5 and Go 1.27.0 from `go.mod`; the native gate, uncached CGO-disabled tests, and six-target compilation passed. See [hosted platform evidence](platforms.md#hosted-ci-evidence) for settings and limits. The tag-triggered draft workflow has not run.

## Build release assets locally

Use native Linux/amd64 with Go, GNU Make, Git, Bash, GNU tar/coreutils, and gzip. Start from a clean committed tree. Stable versions use `X.Y.Z`; prereleases use `X.Y.Z-alpha.N`, `-beta.N`, or `-rc.N`, with N starting at 1. The owner selected `0.1.0-rc.1` for the first candidate; its [changelog section](../CHANGELOG.md#010-rc1) is prepared, and publication is pending:

```sh
make release VERSION=0.1.0-rc.1
```

This runs the full gate before [the packaging script](../tools/release.sh). The script exports the exact Git commit, builds from that snapshot with CGO disabled and `GOAMD64=v1`, and checks the packaged version/help. Dirty trees, malformed versions, failed builds, and existing output are refused. Temporary files are cleaned on failure; completed asset directories are never overwritten.

The output directory `bin/releases/<version>/` contains:

- `yoinkdl_<version>_linux_amd64.tar.gz`: executable, project/runtime licenses, security policy, changelog, installation text, and `SOURCE.txt` identifying the commit/toolchain/source archive.
- `yoinkdl_<version>_source.tar.gz`: matching committed sources, including build files and tests.
- `SHA256SUMS`: checksums for both archives, verified before final output appears.

Archive ownership, modes, ordering, and timestamps are normalized. Tests verify identical archives when the same snapshot is rebuilt with the same toolchain. This does not promise identical binaries across Go versions or toolchain configurations. Artifacts are unsigned; the checksum file detects corruption and is not a separate publisher-authentication mechanism.

## First candidate verification

Local `0.1.0-rc.1` archives were built and verified on October 12, 2026, from clean commit `5fae374cd31429f5d8653c6c497dcf0d21865c7e`. `make release VERSION=0.1.0-rc.1` passed the full gate and produced the three assets under `bin/releases/0.1.0-rc.1/` using `go1.27.2-X:nodwarf5` on Linux/amd64. Core ordinary/race suites reused matching cached results; packaging tests reran. An initial sandboxed attempt stopped at the packaging test's blocked localhost listener; the complete command passed outside that sandbox.

| Archive | SHA-256 |
|---|---|
| `yoinkdl_0.1.0-rc.1_linux_amd64.tar.gz` | `2a7dfc818b8e4da510fb00e6ba5bfa8b8d7df1445c099084fd8ac570b9c75027` |
| `yoinkdl_0.1.0-rc.1_source.tar.gz` | `dfedc17494a08ca91280e844249f1cafd2c66f5c0fd83c78d51aaa1df336e304` |

`sha256sum -c SHA256SUMS` passed. The uncompressed source archive matched `git archive` for that revision byte for byte. Packaged license/changelog/security files matched the commit, and `SOURCE.txt` identified the same revision and toolchain. Build metadata confirmed CGO disabled, Linux/amd64, and `GOAMD64=v1`.

An isolated installation using `install -m 0755` passed version/help checks and downloaded a deterministic 9,437,201-byte local fixture with one and four connections. The four-connection run made a one-byte probe and three whole-chunk requests. Both outputs matched SHA-256 `81e04b567859b9852a829e4f54b677634ee7fcda678a2ddb90012b4c632c62a2`, reported completed/published JSON results with exit zero, and removed their partial/manifest files.

The maintainer can tag `5fae374cd31429f5d8653c6c497dcf0d21865c7e` as `v0.1.0-rc.1` after pushing the prepared commits. Later evidence-only documentation commits are outside these source archives. If the release revision changes, rebuild and verify that revision in a separate clean checkout; preserve existing candidate output. The hosted draft will build with Go 1.27.0, so verify its own checksums and installation rather than expecting binary equality with this local build. No tag, draft, or public release was created during local preparation.

## Draft-release workflow

[Draft release](../.github/workflows/release.yml) runs for pushed `v*` tags. It validates the tag, requires nonempty notes beneath the exact `## [<version>]` heading in [CHANGELOG.md](../CHANGELOG.md), runs `make release`, and creates a GitHub draft with both archives and checksums. A prerelease version produces a prerelease draft. Missing notes or failed validation stop before any draft is created. Existing releases are not overwritten.

Before creating a tag, move the relevant Unreleased entries into a matching version section, review the changes, and commit them. Review the generated draft and its assets before publishing. The workflow needs `contents: write` only for its release job; it does not use a personal token or create tags. This implementation does not push commits/tags, dispatch Actions, or create a remote release itself.

An interrupted upload can leave an incomplete draft. Inspect it before retrying; the workflow will not overwrite an existing release.

## Install a published binary

After a maintainer publishes a release, download its binary archive, matching source archive, and checksum file from the same release. Replace this example version with that published version:

```sh
version=0.1.0-rc.1
sha256sum -c SHA256SUMS
tar -xzf "yoinkdl_${version}_linux_amd64.tar.gz"
cd "yoinkdl_${version}_linux_amd64"
./yoinkdl version
mkdir -p "$HOME/.local/bin"
install -m 0755 yoinkdl "$HOME/.local/bin/yoinkdl"
```

Ensure `$HOME/.local/bin` is on PATH. The archive's `SOURCE.txt` identifies the corresponding source archive. Other native platforms, broader filesystem durability, signing, and package-manager distribution remain outside the current release evidence; see [platform qualification](platforms.md).
