# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) is published as the latest stable release for Linux/amd64. Milestones A through D are complete within that qualified scope: implementation, seven-criterion acceptance review, hosted gates, packaging, public downloads, and installed HTTP/HTTPS checks have recorded evidence. The `0.1.0-rc.1` prerelease is preserved separately.

The standalone CLI provides sequential/parallel transfers, safe resume, retries, rate caps, text/JSON output, cancellation, and verified file publication. MPL 2.0 and public GitHub Issues reporting remain the policies. v0.2's daemon and persistent queue are planned, not implemented; other native platforms and wider durability remain unqualified.

## Handoff

- The authorized continuation published the verified stable draft at `2026-10-13T03:00:30Z` with `draft: false` and `prerelease: false`; the public Latest endpoint resolves to `v0.1.0`. Release notes and all three asset identities, sizes, and digests are unchanged.
- Preserve annotated tag `v0.1.0` at `31330c6b07d33af148341bdfb346c0e139082fdb` and `v0.1.0-rc.1` at `5fae374cd31429f5d8653c6c497dcf0d21865c7e`. The earlier prerelease's notes/assets remain unchanged. Later documentation commits are outside each source archive; do not replace released files with builds from current main.
- [Stable publication](docs/releases.md#stable-publication) records durable evidence and installation links. Unauthenticated files under ignored `bin/releases/0.1.0-public/` match the verified Go 1.27.0 files in `bin/releases/0.1.0-hosted/` byte for byte. The local Go 1.27.2 build under `bin/releases/0.1.0/` has a different binary checksum; keep each checksum file paired with its own archives.
- The public binary passed isolated installation, version/help, exact 9,437,201-byte HTTP/HTTPS downloads with one and four connections, successful JSON and cleanup, and untrusted-certificate refusal without HTTP requests or downloaded data. Source snapshots, licenses, metadata, and CGO-disabled Linux/amd64 build settings were rechecked.
- [Stable release run 38338657349](https://github.com/devkcud/ydl-re/actions/runs/38338657349) and [CI run 38338657280](https://github.com/devkcud/ydl-re/actions/runs/38338657280) passed at the tag with Go 1.27.0 on Ubuntu 24.04.5. The later [handoff CI 38339235950](https://github.com/devkcud/ydl-re/actions/runs/38339235950) passed at `0600c05`. Real Linux TLS qualification is in `cmd/yoinkdl/https_linux_test.go`; all seven acceptance criteria are mapped in the [stable review](docs/releases.md#stable-readiness-review).
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directories, private single-link mutable partials, strict representation identity, atomic checkpoints, verified-ready recovery, explicit overwrite, and stable locks. This step changes publication state and documentation only. Release-task push authorization does not cover unrelated feature work.

## Validation

- Prepublication `gh release view` and `git ls-remote` confirmed both tag targets and the unchanged verified stable draft. `gh release edit v0.1.0 --repo devkcud/ydl-re --draft=false --prerelease=false --latest --verify-tag` succeeded.
- `python3 /tmp/yoinkdl-public-stable-check.py` passed outside the sandbox. Requests used no GitHub credentials or cookies. The public page, release API, Latest API, and all three canonical downloads succeeded; stable metadata, exact notes, asset IDs, sizes, and digests matched the verified draft. Public bytes match the hosted files. Separate API checks confirmed the published prerelease and its assets remain unchanged.
- The public check invoked `/tmp/yoinkdl-stable-smoke.py` against `bin/releases/0.1.0-public/`: checksums, exact Git archive identity, licenses/metadata, isolated `install -m 0755`, version/help, HTTP/HTTPS one-/four-connection downloads, and TLS refusal passed. Temporary details are `/tmp/yoinkdl-stable-verification-s_f7sqg5/verification.json` and `/tmp/yoinkdl-stable-public-api.json`; durable results and hashes are in the release guide.
- Existing local/hosted full gates, race tests, CGO-disabled tests, six-target compilation, and HTTPS qualification remain valid for the unchanged code. No redundant local gate was run for this publication/documentation step. `python3 /tmp/yoinkdl-doccheck.py` passed: 24 Markdown files, 293 local links/anchors, 32 shell blocks, five JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. Original-design `<id>` placeholders were normalized for syntax checking; its intentional hard-break spaces remain unchanged.
- Linux/arm64/macOS native execution, Windows storage, full-disk/network-filesystem/power-loss qualification, and interrupted remote upload/retry behavior remain outside the evidence. Compilation or stable versioning does not extend these claims.

## Next steps

1. Check [GitHub Issues](https://github.com/devkcud/ydl-re/issues) for post-release defects and reproduce any reports with deterministic fixtures before changing the engine.
2. If no release fix takes priority, define the v0.2 contracts tracked as R03/R04 in [open decisions](docs/decisions.md#release-and-compatibility-decisions): local IPC and permissions, durable queue/state recovery, transfer scheduling, and shared bandwidth. Document an ordered implementation plan before adding a daemon; standalone downloads must remain independent.
3. Implement v0.2 in those dependency-ordered slices with focused tests and handoffs. Broader native support requires separate runtime qualification; TUI and extra protocols remain later work.

## Open decisions and blockers

No v0.1 release blocker remains within the initial Linux/amd64 scope. The next feature milestone is v0.2, whose IPC, queue persistence, scheduling, and daemon-wide bandwidth contracts are unresolved. Broader native qualification, durability, signing, and package-manager distribution remain separate work.
