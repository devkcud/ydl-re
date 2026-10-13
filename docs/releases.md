# CI and releases

[Documentation index](README.md)

The [0.1.0-rc.1 prerelease](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0-rc.1) provides Linux/amd64 binaries, matching sources, and checksums. Project sources use [MPL 2.0](../LICENSE); the binary also carries the Go runtime's [BSD notice](../LICENSES/Go-BSD-3-Clause.txt). Security reports use public [GitHub Issues](../SECURITY.md). No stable version has been published by this work.

## Continuous integration

[CI](../.github/workflows/ci.yml) runs on pull requests, main-branch pushes, and manual dispatch. Ubuntu 24.04 runs the existing `make check` gate and an uncached CGO-disabled test run. A separate job cross-builds the CLI and test programs for Linux, macOS, and Windows on amd64/arm64. Foreign test programs are compiled only; Windows storage remains unsupported.

Both workflows pin [checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1) and [setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0) to full commit IDs. The Go version comes from `go.mod`; dependency caching is disabled because the project has no external Go modules or `go.sum`. Checkout does not persist credentials. CI has read-only repository permissions, time limits, and cancellation of superseded runs.

[CI run 38294238535](https://github.com/devkcud/ydl-re/actions/runs/38294238535) passed on October 12, 2026, for commit `1ce24810b7b6a05607c30ab37ff725758e29ebb8`. Both jobs used Ubuntu 24.04.5 and Go 1.27.0 from `go.mod`; the native gate, uncached CGO-disabled tests, and six-target compilation passed. See [hosted platform evidence](platforms.md#hosted-ci-evidence) for settings and limits. The later [hosted candidate report](#hosted-candidate-verification) verifies tag-triggered packaging and uploaded assets.

## Build release assets locally

Use native Linux/amd64 with Go, GNU Make, Git, Bash, GNU tar/coreutils, and gzip. Start from a clean committed tree. Stable versions use `X.Y.Z`; prereleases use `X.Y.Z-alpha.N`, `-beta.N`, or `-rc.N`, with N starting at 1. The first candidate is published as `0.1.0-rc.1`; this command demonstrates local packaging without modifying that release:

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

The annotated tag `v0.1.0-rc.1` now points to `5fae374cd31429f5d8653c6c497dcf0d21865c7e`. Later evidence-only documentation commits are outside these source archives. Preserve existing candidate output and the published tag identity. The hosted candidate below uses the same sources with Go 1.27.0 and has its own binary checksum.

## Hosted candidate verification

After explicit owner authorization, an atomic push advanced main to `8a6d5b95de472f42a2f043e628fc5f8788470e89` and created `v0.1.0-rc.1` at the verified revision. [Draft run 38335647469](https://github.com/devkcud/ydl-re/actions/runs/38335647469) passed on October 13, 2026 UTC (October 12 in America/Sao_Paulo), using Ubuntu 24.04.5, runner image `20261004.327.1`, and Go 1.27.0 without Go experiments. The full gate, ordinary/race packaging tests, tag/changelog guard, packaging, and all uploads passed. [CI run 38335647491](https://github.com/devkcud/ydl-re/actions/runs/38335647491) also passed the native gate, uncached CGO-disabled tests, and six-target compilation for the pushed main revision.

The [YoinkDL 0.1.0-rc.1 release](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0-rc.1) was first verified as a prerelease draft, then published after those checks. Its body exactly matches the tagged changelog section. All three assets report uploaded status; their downloaded sizes and SHA-256 values match GitHub metadata. Authenticated draft downloads are retained separately under `bin/releases/0.1.0-rc.1-hosted/`.

| Hosted asset | SHA-256 |
|---|---|
| `yoinkdl_0.1.0-rc.1_linux_amd64.tar.gz` | `89cd886bd10d11e2645e0c3b6a3bc920c544bbc0dce992acc15d9674553de835` |
| `yoinkdl_0.1.0-rc.1_source.tar.gz` | `dfedc17494a08ca91280e844249f1cafd2c66f5c0fd83c78d51aaa1df336e304` |
| `SHA256SUMS` | `eae8848c7bfadf3aa35a229b5a74746b0ae044f697a9f5b80876819b1f747c1c` |

`sha256sum -c SHA256SUMS` passed for the downloaded assets. The source archive matches the local candidate archive and its uncompressed contents match `git archive` at `5fae374cd31429f5d8653c6c497dcf0d21865c7e` exactly. Packaged licenses, changelog, security policy, and `SOURCE.txt` match that revision. Binary metadata confirms Go 1.27.0, CGO disabled, Linux/amd64, and `GOAMD64=v1`.

An isolated `install -m 0755` passed version/help and the same 9,437,201-byte one-/four-connection fixture downloads described above, including exact hashes, completed/published JSON, and partial-state cleanup. No application or workflow change was needed. These initial checks used authenticated access to the draft; public checks follow below. Interrupted upload/retry behavior and platforms beyond Linux/amd64 remain unqualified.

## Publication and public downloads

The verified candidate was published at `2026-10-13T02:24:50Z` (October 12 in America/Sao_Paulo) with `gh release edit v0.1.0-rc.1 --repo devkcud/ydl-re --draft=false --prerelease --latest=false --verify-tag`. It remains a prerelease. The tag, release notes, asset IDs, sizes, and SHA-256 digests were preserved.

Unauthenticated HTTPS requests returned success for the release page, the public release API, and all three canonical asset URLs. Public downloads under `bin/releases/0.1.0-rc.1-public/` match the verified hosted files byte for byte, including SHA256SUMS. The checksums above therefore apply to the published release as well.

The publicly downloaded archives passed checksum verification, exact source-snapshot comparison, packaged license/metadata checks, and isolated installation. Version/help and one-/four-connection downloads reproduced the same 9,437,201-byte fixture hash, successful JSON results, and partial-state cleanup. The public API reports `draft: false`, `prerelease: true`, and the publication time above. This qualifies the published Linux/amd64 candidate; stable promotion still requires candidate review.

## Draft-release workflow

[Draft release](../.github/workflows/release.yml) runs for pushed `v*` tags. It validates the tag, requires nonempty notes beneath the exact `## [<version>]` heading in [CHANGELOG.md](../CHANGELOG.md), runs `make release`, and creates a GitHub draft with both archives and checksums. A prerelease version produces a prerelease draft. Missing notes or failed validation stop before any draft is created. Existing releases are not overwritten.

Before creating a tag, move the relevant Unreleased entries into a matching version section, review the changes, and commit them. Review the generated draft and its assets before publishing. The workflow needs `contents: write` only for its release job; it does not use a personal token or create tags. Local packaging does not push commits/tags, dispatch Actions, or create a remote release.

An interrupted upload can leave an incomplete draft. Inspect it before retrying; the workflow will not overwrite an existing release.

## Install a published binary

For Linux/amd64, save the published [binary archive](https://github.com/devkcud/ydl-re/releases/download/v0.1.0-rc.1/yoinkdl_0.1.0-rc.1_linux_amd64.tar.gz), [source archive](https://github.com/devkcud/ydl-re/releases/download/v0.1.0-rc.1/yoinkdl_0.1.0-rc.1_source.tar.gz), and [SHA256SUMS](https://github.com/devkcud/ydl-re/releases/download/v0.1.0-rc.1/SHA256SUMS) in an empty directory, then run:

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
