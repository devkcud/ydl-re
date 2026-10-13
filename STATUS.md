# Project status

Updated: October 12, 2026 (America/Sao_Paulo).

## Current state

Milestones A, B, and C are implemented. Milestone D includes aggregate rate caps, versioned JSON/text output, the local security review, native Linux/amd64 evidence, and passing hosted CI/release tooling. The full gate, uncached CGO-disabled tests, and six-target compilation pass with the minimum Go 1.27.0 toolchain.

[YoinkDL 0.1.0-rc.1](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0-rc.1) is published as a Linux/amd64 prerelease. Candidate review and local HTTPS qualification passed; `0.1.0` notes are prepared for the same downloader behavior. Stable archives and hosted verification are next; no stable tag or release exists. MPL 2.0 and public GitHub Issues reporting remain the selected policies.

## Handoff

- The candidate review checked all GitHub Issues and current CI at `4829966`; no issues were returned and both CI jobs passed. This is absence of reports, not independent field-validation evidence. [Stable readiness review](docs/releases.md#stable-readiness-review) maps all seven criteria to actual tests and recorded results. No unresolved integrity failure was found within the documented Linux/amd64 scope.
- `a17767a` adds real TLS handshakes through `cmd/yoinkdl/https_linux_test.go`: trusted sequential/parallel downloads and untrusted-certificate refusal. Per-process CA files keep host trust stores unchanged; the tests are Linux-only because they use Linux certificate discovery. No product code changed. Focused, CGO-disabled, race, and full-gate checks passed.
- `CHANGELOG.md` now contains exact `## [0.1.0]` notes while preserving the original candidate section. Runtime, build, packaging, workflow, and license sources are unchanged from the candidate; stable differences are qualification tests, version labeling, and documentation. Commit the preparation before packaging so source archives identify a clean revision.
- Continuing the owner-authorized release work published the verified draft at `2026-10-13T02:24:50Z` (October 12 locally). It remains a prerelease and was not marked Latest. The release notes and all three asset IDs, sizes, and digests are unchanged.
- Annotated tag `v0.1.0-rc.1` still resolves to `5fae374cd31429f5d8653c6c497dcf0d21865c7e`. Preserve the tag and release assets. Later documentation commits are outside the source archive; do not replace candidate files with builds from current main.
- [Publication evidence](docs/releases.md#publication-and-public-downloads) records the public checks. Unauthenticated downloads are retained under ignored `bin/releases/0.1.0-rc.1-public/` and match the verified Go 1.27.0 files in `bin/releases/0.1.0-rc.1-hosted/` byte for byte. The earlier local Go 1.27.2 build is separate under `bin/releases/0.1.0-rc.1/`; keep each checksum file with its own archives.
- [Draft run 38335647469](https://github.com/devkcud/ydl-re/actions/runs/38335647469) passed the full gate, packaging, and uploads at the release revision. [CI run 38336140333](https://github.com/devkcud/ydl-re/actions/runs/38336140333) passed both jobs at `a88b2420c59cfa92bf657034f6f6080e1198c479`. The release runner used Ubuntu 24.04.5, image `20261004.327.1`, Go 1.27.0, and no Go experiments.
- [CI and releases](docs/releases.md) owns clean-snapshot packaging, changelog headings, draft creation, and installation. Public installation instructions now link directly to all three published assets. No application, workflow, repository-setting, or license changes were needed.
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directory, private single-link mutable partials, strict representation validation, atomic checkpoints, offline verified-ready recovery, explicit overwrite, and stable lock inodes. Output failures retain verified files.
- Linux/arm64/macOS remain unqualified natively; Windows storage is unsupported. Hosted compilation adds no filesystem-specific durability guarantees. The original design is unchanged. Push authorization applies to this release task; the default no-push policy still applies to unrelated work.

## Validation

- `go test ./cmd/yoinkdl -run '^TestProcessHTTPS$' -count=1 -v`, the CGO-disabled variant, and the `-race` variant passed. `make check` then passed formatting, vet, ordinary/race tests, and build; CLI and packaging tests reran while unchanged core packages used matching cached results. The new HTTPS test has local Go 1.27.2 evidence; hosted Go 1.27.0 execution is still pending.
- Before publication, `gh release view` and `git ls-remote` confirmed the same draft, notes, three verified assets, and tag target. `gh release edit v0.1.0-rc.1 --repo devkcud/ydl-re --draft=false --prerelease --latest=false --verify-tag` published the existing release successfully.
- `python3 /tmp/yoinkdl-public-release-check.py` passed outside the sandbox. Its HTTP requests used no GitHub credentials or cookies. The public page, release API, and three canonical download URLs succeeded; metadata reports `draft: false`, `prerelease: true`, and the expected publication time. Asset IDs, sizes, digests, release notes, and downloaded bytes match the prepublication snapshot.
- The public check invoked `/tmp/yoinkdl-hosted-release-smoke.py` against `bin/releases/0.1.0-rc.1-public/`: checksums, exact Git source snapshot, licenses/metadata, Go 1.27.0/CGO-disabled settings, isolated `install -m 0755`, version/help, and one-/four-connection downloads of 9,437,201 bytes passed with exact hashes, successful JSON, and partial-state cleanup. Temporary detail is `/tmp/yoinkdl-hosted-rc1-verification-w3qzk0cj/verification.json`; the public API snapshot is `/tmp/yoinkdl-rc1-public-api.json`. Durable hashes and results are in the release guide.
- The release workflow's exact Bash tag/changelog guard accepted `v0.1.0` and selected only the eight stable entries and scope notes. The published candidate's changelog section is unchanged. `python3 /tmp/yoinkdl-doccheck.py` passed: 24 Markdown files, 291 local links/anchors, 32 shell blocks, five JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. The original design's illustrative `<id>` shell placeholders were normalized for syntax checking; its intentional hard-break spaces remain unchanged.
- Native foreign-platform execution, full-disk/network-filesystem/power-loss qualification, and interrupted remote upload/retry behavior remain unverified. Evidence and reproduction commands remain in [testing](docs/testing.md), [platforms](docs/platforms.md), and [releases](docs/releases.md).

## Next steps

1. Run `make release VERSION=0.1.0` from the clean stable-preparation commit. Verify archive checksums, source identity, installed version/help, and HTTP/HTTPS downloads; record the exact revision and results here.
2. Create the stable draft through the existing tag workflow and verify its assets plus the new HTTPS fixture on hosted Go 1.27.0 before publication. Preserve the published candidate. Broader native support needs separate runtime evidence; configuration files and daemon/TUI work stay optional/later.

## Open decisions and blockers

The first prerelease, candidate review, local HTTPS qualification, and stable notes are complete. Stable archives and hosted verification remain. No reported regression is awaiting reproduction. Broader native qualification, durability, signing, and package-manager distribution remain outside current evidence.
