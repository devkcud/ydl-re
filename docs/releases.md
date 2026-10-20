# CI and releases

[Documentation index](README.md)

Stable [YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) provides Linux/amd64 binaries, matching sources, and checksums. Project sources use [MPL 2.0](../LICENSE); the binary also carries the Go runtime's [BSD notice](../LICENSES/Go-BSD-3-Clause.txt). Security reports use public [GitHub Issues](../SECURITY.md). The earlier `0.1.0-rc.1` prerelease remains available separately.

## Continuous integration

[CI](../.github/workflows/ci.yml) runs on pull requests, main-branch pushes, and manual dispatch. Ubuntu 24.04 runs the existing `make check` gate and an uncached CGO-disabled test run. A separate job cross-builds the CLI and test programs for Linux, macOS, and Windows on amd64/arm64. Foreign test programs are compiled only; Windows storage remains unsupported.

Both workflows pin [checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1) and [setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0) to full commit IDs. The Go version comes from `go.mod`; dependency caching is disabled because the project has no external Go modules or `go.sum`. Checkout does not persist credentials. CI has read-only repository permissions, time limits, and cancellation of superseded runs.

[CI run 38294238535](https://github.com/devkcud/ydl-re/actions/runs/38294238535) passed on October 12, 2026, for commit `1ce24810b7b6a05607c30ab37ff725758e29ebb8`. Both jobs used Ubuntu 24.04.5 and Go 1.27.0 from `go.mod`; the native gate, uncached CGO-disabled tests, and six-target compilation passed. See [hosted platform evidence](platforms.md#hosted-ci-evidence) for settings and limits. The later [hosted candidate report](#hosted-candidate-verification) verifies tag-triggered packaging and uploaded assets.

## Build release assets locally

Use native Linux/amd64 with Go, GNU Make, Git, Bash, GNU tar/coreutils, and gzip. Start from a clean committed tree. Stable versions use `X.Y.Z`; prereleases use `X.Y.Z-alpha.N`, `-beta.N`, or `-rc.N`, with N starting at 1. This command demonstrates local packaging without modifying the published release:

```sh
make release VERSION=0.1.0
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

The publicly downloaded archives passed checksum verification, exact source-snapshot comparison, packaged license/metadata checks, and isolated installation. Version/help and one-/four-connection downloads reproduced the same 9,437,201-byte fixture hash, successful JSON results, and partial-state cleanup. The public API reports `draft: false`, `prerelease: true`, and the publication time above. These checks qualified the published Linux/amd64 candidate; the subsequent stable review is recorded below.

## Stable readiness review

The candidate review on October 12, 2026 (America/Sao_Paulo) found no GitHub Issues in any state. This provides no independent field-testing evidence and supplies no reported regression to reproduce. [CI run 38337336004](https://github.com/devkcud/ydl-re/actions/runs/38337336004) passed both jobs at `48299669d3e930d18d296c1568b82807d370152e`. The production, build, packaging, workflow, and license sources match `v0.1.0-rc.1`; later changes add documentation and HTTPS qualification tests.

The [seven acceptance criteria](testing.md#core-integrity-acceptance) have the following evidence within the existing Linux/amd64 platform limits:

| Criterion | Reviewed evidence |
|---|---|
| Correct direct HTTP(S) bytes | [HTTP fixtures](../internal/transfer/download_test.go) and [Linux HTTPS subprocesses](../cmd/yoinkdl/https_linux_test.go) verify exact content. Trusted sequential/ranged TLS succeeds; untrusted TLS fails before HTTP requests or downloaded data. |
| Resume across process termination | [Sequential](../cmd/yoinkdl/resume_test.go) and [parallel](../cmd/yoinkdl/range_resume_test.go) tests kill writers after checkpoints, restart, and compare complete fixture hashes. |
| No silent resource-version mixing | [Worker failures](../internal/transfer/workers_test.go) and [resume refusals](../internal/transfer/resume_test.go) reject changed validators/bounds and preserve existing verified state. |
| Wrong/incomplete lengths detected | The HTTP short-body and worker short/oversized/broken-body cases preserve incomplete state and refuse final publication. |
| Verified atomic publication | [Storage publication](../internal/storage/file_test.go) and [range recovery](../internal/storage/range_checkpoint_test.go) cover no-overwrite links, deliberate replacement, conflicts, ready-state recovery, and coverage/byte checks under the qualified Unix policy. |
| Simultaneous writers excluded | Storage ownership tests admit one file owner; [range writer tests](../internal/storage/range_writer_test.go) admit one claimant per chunk and verify exact concurrent output. |
| Integration/race gates pass | Hosted Go 1.27.0 native/race and CGO-disabled runs passed, including the new TLS fixture. Focused ordinary, CGO-disabled, race, and full-gate checks also passed locally on Go 1.27.2. |

The HTTPS test was added in `a17767a`; it uses isolated per-process CA files without changing the host trust store or product transport settings. Native Linux/arm64/macOS, Windows storage, network filesystems, full-disk/power-loss behavior, and interrupted remote uploads remain outside these claims. No unresolved integrity failure was found on the reviewed paths. Lack of issue reports alone was not used as an acceptance result.

The exact `## [0.1.0]` section in [CHANGELOG.md](../CHANGELOG.md#010) supplies the published notes, preserving the prerelease section. Annotated tag `v0.1.0` points to `31330c6b07d33af148341bdfb346c0e139082fdb`. The same revision and tag were pushed atomically for hosted CI and draft creation, followed by verified publication. Preserve both published releases and their tags/assets.

## Stable archive verification

`make release VERSION=0.1.0` passed the full gate and built local archives from clean revision `31330c6b07d33af148341bdfb346c0e139082fdb`. The files under `bin/releases/0.1.0/` use Go 1.27.2 with `nodwarf5`; matching core test results were cached, and packaging tests reran.

| Local stable archive | SHA-256 |
|---|---|
| `yoinkdl_0.1.0_linux_amd64.tar.gz` | `98c2c1b738333f320738ff5524024a4889651a309a329a4ba0421a5acb3713e7` |
| `yoinkdl_0.1.0_source.tar.gz` | `756afe74e9488e8bfb06e3a07093a45429d1683fe3bd4f4d303a78ba9f7c99d0` |

Checksums, exact `git archive` source identity, packaged licenses/metadata, CGO-disabled Linux/amd64 settings, and an isolated `install -m 0755` passed. The installed `yoinkdl 0.1.0` passed version/help and 9,437,201-byte fixture downloads over HTTP and HTTPS with one and four connections, matching the earlier fixture hash. Untrusted TLS was rejected without an HTTP request, output/partial data, or manifest. The local HTTPS smoke fixture used a temporary OpenSSL certificate and per-process CA paths; no host trust settings changed.

[Stable draft run 38338657349](https://github.com/devkcud/ydl-re/actions/runs/38338657349) passed the full gate, packaging, and uploads with Go 1.27.0 on Ubuntu 24.04.5. [CI run 38338657280](https://github.com/devkcud/ydl-re/actions/runs/38338657280) passed native/race tests, uncached CGO-disabled tests, and six-target compilation at the same revision. These runs include the new Linux HTTPS fixture.

The [YoinkDL 0.1.0 release](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) was verified as a stable draft before publication, with notes exactly matching its tagged changelog section. All three assets are uploaded; downloaded sizes and SHA-256 digests match GitHub metadata. The draft downloads are retained separately under `bin/releases/0.1.0-hosted/`.

| Hosted stable asset | SHA-256 |
|---|---|
| `yoinkdl_0.1.0_linux_amd64.tar.gz` | `bdf0deb8d5a52d8d7c3a31785b658d029e52e75a0f3aa6b344e9cec464c9b6d8` |
| `yoinkdl_0.1.0_source.tar.gz` | `756afe74e9488e8bfb06e3a07093a45429d1683fe3bd4f4d303a78ba9f7c99d0` |
| `SHA256SUMS` | `48ed2cf34506be56d08e238605e889526deb16df728fd0d902e634b17ab7f68b` |

The downloaded source archive matches the local source archive and exact Git snapshot. The hosted binary identifies Go 1.27.0, CGO disabled, Linux/amd64, and `GOAMD64=v1`; installed version/help, all four HTTP/HTTPS fixture downloads, exact hashes, successful JSON, partial-state cleanup, and untrusted TLS refusal passed again. The existing assets were then published unchanged, preserving `0.1.0-rc.1`.

## Stable publication

The verified stable release was published at `2026-10-13T03:00:30Z` with `gh release edit v0.1.0 --repo devkcud/ydl-re --draft=false --prerelease=false --latest --verify-tag`. The public API reports `draft: false`, `prerelease: false`, and that publication time; its Latest endpoint resolves to the same `v0.1.0` release.

Unauthenticated HTTPS requests succeeded for the release page, release API, Latest API, and all three canonical asset URLs. Public files under `bin/releases/0.1.0-public/` match the verified hosted files byte for byte. Release notes, asset IDs, sizes, and digests are unchanged, so the hosted stable checksums above apply to the public release.

The public binary passed an isolated `install -m 0755`, version/help, and the same 9,437,201-byte HTTP/HTTPS fixture with one and four connections. Exact hashes, completed/published JSON results, partial-state cleanup, and untrusted-certificate refusal passed. Source archive, licenses, metadata, and build settings were rechecked. The existing `0.1.0-rc.1` remains a separate published prerelease with unchanged notes, asset identities, sizes, and digests. Milestones A through D are complete for the qualified Linux/amd64 scope; future platforms and wider durability claims need separate evidence.

## v0.2 local qualification

Final E6 runtime/platform gates passed on October 19, 2026, against clean runtime/test revision `5c00f2e1e522268c27ab89aba7d1f479fa12950f`. The [platform report](platforms.md#native-linux-evidence) records Linux/amd64, Go `1.27.2-X:nodwarf5`, ext4 ordinary/race execution, an uncached whole-suite CGO-disabled tmpfs run, and CLI/eight-package test compilation for all six targets. Foreign test programs were not executed.

The [E6 testing report](testing.md#current-evidence) covers forced death, retained queue capacity, conditional resume, publication ambiguity, concurrency, and shared budgets. [Security evidence](security.md#review-evidence) includes private queue ownership, strict IPC, source redaction, uncertain commits, and preservation through forget/restart. Existing standalone transfer, TLS, output, storage, and packaging suites remain passing regression gates.

The [0.2.0-rc.1 notes](../CHANGELOG.md#020-rc1) belong to clean candidate revision `34041c372519ac751bb9d6ec1ecda56184d13161`. Its runtime/test/build sources are unchanged from the E6 gate revision above. The candidate was built with:

```sh
TMPDIR="$PWD/bin/t.MBOfTk" make release VERSION=0.2.0-rc.1
```

The full packaging gate passed, rerunning all seven filesystem/process packages ordinarily and with the race detector; only range planning reused cached results. The three assets are retained under ignored `bin/releases/0.2.0-rc.1/`. Packaging used Go `1.27.2-X:nodwarf5`, `CGO_ENABLED=0`, `GOAMD64=v1`, and the committed source snapshot. Later evidence-only documentation commits are not part of this candidate's source identity.

| Archive | SHA-256 |
|---|---|
| `yoinkdl_0.2.0-rc.1_linux_amd64.tar.gz` | `609ef91668a87ef91a94f30b53c1111416af807e0666417150c420cf1176a3b9` |
| `yoinkdl_0.2.0-rc.1_source.tar.gz` | `d03d4de60c475610e9a32957f6a27c2ab09d0ee8e52714b5b1de8ffa35480f02` |

Both archives match `SHA256SUMS`; the decompressed source archive matches `git archive` of the candidate revision byte for byte. Packaged licenses, security policy, changelog, source metadata, and binary build settings were checked. An isolated `install -m 0755` under `bin/r.7o9q_lmi/` passed version/help and sequential/four-worker HTTP and trusted HTTPS downloads. Every output matched the 9,437,201-byte fixture's SHA-256, `39202f5960cff6f21401ff53547fd309d1a4dc3a88f9498240e38807b0208e01`; an untrusted certificate was refused without final publication.

The installed daemon passed explicit startup, private source retention and terminal clearing, safe views, pause, clean stop/restart, persisted concurrency, restart-selected shared cap, and conditional resume from a 131,072-byte prefix to the exact fixture. Forget preserved the completed file; readmission refused that existing file without HTTP traffic or inode changes. Cancellation and forget retained a matching partial prefix. Clean shutdown completed, and synthetic URL tokens were absent from command output and daemon logs.

Local evidence and rerunnable verification scripts are in `bin/e6-qualification.0Stkvu/`: `release.log`, `installed-candidate.json`, `installed-candidate.log`, and `yoinkdl-verify-release.py`. Run the latter from the repository root with `0.2.0-rc.1`; it checks the source revision recorded in the package and creates fresh isolated fixtures. All 18 existing v0.1 archive/checksum files and both existing tag objects remain unchanged.

E1 through E6 completed local Linux/amd64 qualification before the owner authorized the hosted release below. The local archives remain separate from hosted/public downloads. Foreign native execution and power-loss durability remain unqualified.

## v0.2 hosted qualification and publication

The owner explicitly authorized pushing and publishing `v0.2.0-rc.1` after hosted validation. Main was already pushed at `ae8339a9aa507dd526719318e9f9ea6d934df3a9`. Annotated tag object `31bebc5dfc543d318aa47b5ec5fb9c1f69657fa7` was created and pushed at the verified candidate source `34041c372519ac751bb9d6ec1ecda56184d13161`; no later documentation revision was substituted.

[CI run 39338571399](https://github.com/devkcud/ydl-re/actions/runs/39338571399) passed the native full/race gate, uncached CGO-disabled tests, and six-target CLI/test compilation at the pushed main revision. [Draft run 39338981764](https://github.com/devkcud/ydl-re/actions/runs/39338981764) passed tag/changelog validation, the full packaging gate, and all uploads at the candidate tag. Both used Ubuntu 24.04.5, runner image `20261012.334.1`, and Go `1.27.0` with no Go experiments. Foreign binaries were compiled only; the hosted logs add no filesystem-specific or power-loss guarantee.

The draft's notes matched the tagged `0.2.0-rc.1` changelog section. All three downloaded assets matched GitHub's recorded sizes and SHA-256 digests. The hosted source archive is byte-identical to the local candidate source archive. The binary archive differs from the local Go `1.27.2-X:nodwarf5` build as expected; these are the hosted and published digests:

| Asset | Bytes | SHA-256 |
|---|---|---|
| `yoinkdl_0.2.0-rc.1_linux_amd64.tar.gz` | 3,101,155 | `542759e161799e0276039733e3b70b4bda2d9db3957931af7178af9c9459054b` |
| `yoinkdl_0.2.0-rc.1_source.tar.gz` | 261,445 | `d03d4de60c475610e9a32957f6a27c2ab09d0ee8e52714b5b1de8ffa35480f02` |
| `SHA256SUMS` | 203 | `f60cc1072c0aeaabc28c98d45430b1413b8d92713b5c7c2dcc56958ba1d3d020` |

Separately downloaded hosted and public archives passed the retained `verify-release.py` fixture. Both installed binaries reported `0.2.0-rc.1`, used Go `1.27.0`, `CGO_ENABLED=0`, and `GOAMD64=v1`, and reproduced the exact 9,437,201-byte fixture with one/four workers over HTTP and trusted HTTPS. Untrusted certificates were refused. Daemon checks passed explicit startup, private source retention/clearing, safe views, pause, clean restart, persisted concurrency, conditional resume from 131,072 bytes, final-file preservation/refusal, cancellation, metadata-only forget, and clean shutdown.

Publication completed at `2026-10-19T23:07:45Z` using:

```sh
gh release edit v0.2.0-rc.1 --repo devkcud/ydl-re --draft=false --prerelease --latest=false --verify-tag
```

The [YoinkDL 0.2.0-rc.1 prerelease](https://github.com/devkcud/ydl-re/releases/tag/v0.2.0-rc.1) has release ID `417837910`. Anonymous HTTPS requests succeeded for its page, API, and all three canonical asset URLs. Public files are byte-identical to the verified draft downloads. Notes, asset IDs/sizes/digests, and the tag/source identity are preserved. Both previous releases retain their notes and assets; Latest still resolves to stable `v0.1.0`.

Hosted files are retained under `bin/releases/0.2.0-rc.1-hosted/`, public files under `bin/releases/0.2.0-rc.1-public/`, and logs, API snapshots, checks, and rerunnable verification scripts under `bin/v02-release.8Go8z2/`. The release-specific push/publication exception is complete; unrelated future work remains local under the repository's no-push rule. Stable `0.2.0` promotion has not been requested or performed.

## Draft-release workflow

[Draft release](../.github/workflows/release.yml) runs for pushed `v*` tags. It validates the tag, requires nonempty notes beneath the exact `## [<version>]` heading in [CHANGELOG.md](../CHANGELOG.md), runs `make release`, and creates a GitHub draft with both archives and checksums. A prerelease version produces a prerelease draft. Missing notes or failed validation stop before any draft is created. Existing releases are not overwritten.

Before creating a tag, move the relevant Unreleased entries into a matching version section, review the changes, and commit them. Review the generated draft and its assets before publishing. The workflow needs `contents: write` only for its release job; it does not use a personal token or create tags. Local packaging does not push commits/tags, dispatch Actions, or create a remote release.

An interrupted upload can leave an incomplete draft. Inspect it before retrying; the workflow will not overwrite an existing release.

## Install a published binary

For Linux/amd64, save the stable [binary archive](https://github.com/devkcud/ydl-re/releases/download/v0.1.0/yoinkdl_0.1.0_linux_amd64.tar.gz), [source archive](https://github.com/devkcud/ydl-re/releases/download/v0.1.0/yoinkdl_0.1.0_source.tar.gz), and [SHA256SUMS](https://github.com/devkcud/ydl-re/releases/download/v0.1.0/SHA256SUMS) in an empty directory, then run:

```sh
version=0.1.0
sha256sum -c SHA256SUMS
tar -xzf "yoinkdl_${version}_linux_amd64.tar.gz"
cd "yoinkdl_${version}_linux_amd64"
./yoinkdl version
mkdir -p "$HOME/.local/bin"
install -m 0755 yoinkdl "$HOME/.local/bin/yoinkdl"
```

Ensure `$HOME/.local/bin` is on PATH. The archive's `SOURCE.txt` identifies the corresponding source archive. Other native platforms, broader filesystem durability, signing, and package-manager distribution remain outside the current release evidence; see [platform qualification](platforms.md).

To try the Linux queue prerelease, use its [0.2.0-rc.1 binary archive](https://github.com/devkcud/ydl-re/releases/download/v0.2.0-rc.1/yoinkdl_0.2.0-rc.1_linux_amd64.tar.gz), [matching sources](https://github.com/devkcud/ydl-re/releases/download/v0.2.0-rc.1/yoinkdl_0.2.0-rc.1_source.tar.gz), and [checksums](https://github.com/devkcud/ydl-re/releases/download/v0.2.0-rc.1/SHA256SUMS). Apply the same installation steps with `0.2.0-rc.1` in the archive/directory names. It remains a prerelease; stable `0.1.0` is still Latest.
