# Project status

Updated: October 12, 2026 (America/Sao_Paulo).

## Current state

Milestones A, B, and C are implemented. Milestone D has reviewed Linux/amd64 acceptance evidence, real HTTPS qualification, and passing hosted CI/release tooling. The `0.1.0` stable tag and verified draft are ready; stable publication remains pending. The published `0.1.0-rc.1` prerelease is unchanged and remains available.

The standalone CLI supports sequential/parallel transfers, safe resume, retries, rate caps, text/JSON output, cancellation, and verified file publication. MPL 2.0, Linux/amd64 GitHub Releases, and public GitHub Issues reporting remain the selected policies. Other native platforms and wider filesystem durability remain unqualified.

## Handoff

- Candidate review returned no GitHub Issues in any state and found no runtime changes since `v0.1.0-rc.1`. Absence of reports is not independent field-validation evidence. [Stable readiness review](docs/releases.md#stable-readiness-review) maps all seven acceptance criteria to actual tests and results; no unresolved integrity failure was found in the qualified scope.
- `a17767a` adds `cmd/yoinkdl/https_linux_test.go`: trusted sequential/parallel TLS transfers and untrusted-certificate refusal through subprocesses. Per-process CA files isolate trust without changing the host or product transport. The fixture is Linux-only because it uses Linux certificate discovery. Product behavior is unchanged.
- `31330c6b07d33af148341bdfb346c0e139082fdb` contains the qualification and exact `## [0.1.0]` notes, preserving the candidate section. Main and annotated tag `v0.1.0` were pushed atomically at that revision. Preserve both release tags and their archives; later evidence-only documentation is outside the stable source snapshot.
- [Stable draft run 38338657349](https://github.com/devkcud/ydl-re/actions/runs/38338657349) and [CI run 38338657280](https://github.com/devkcud/ydl-re/actions/runs/38338657280) passed with Go 1.27.0 on Ubuntu 24.04.5, including ordinary/race/CGO-disabled HTTPS tests and six-target compilation. The [YoinkDL 0.1.0 draft](https://github.com/devkcud/ydl-re/releases/tag/untagged-cc8f1c4b4de60a1b50f4) is unpublished and has the stable flag (`isPrerelease: false`). All three assets and the exact notes are verified.
- [Stable archive verification](docs/releases.md#stable-archive-verification) records source identity and hashes. `bin/releases/0.1.0/` contains the local Go 1.27.2 build; `bin/releases/0.1.0-hosted/` contains verified Go 1.27.0 draft downloads. The source archives match; binaries differ with the toolchain. Keep each SHA256SUMS with its own archives.
- Both stable binaries passed isolated installation, version/help, exact 9,437,201-byte HTTP/HTTPS downloads with one and four connections, successful JSON and cleanup, and untrusted-certificate refusal without HTTP requests or downloaded data. Existing published candidate files under the `0.1.0-rc.1*` directories remain intact.
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directories, private single-link mutable partials, strict representation identity, atomic checkpoints, verified-ready recovery, explicit overwrite, and stable locks. No application, workflow, license, or repository-setting changes were needed. Release-task push authorization remains scoped to this work.

## Validation

- `go test ./cmd/yoinkdl -run '^TestProcessHTTPS$' -count=1 -v`, its `CGO_ENABLED=0` variant, and its `-race` variant passed. `make check` passed; CLI/packaging ordinary and race tests reran, while unchanged core suites reused matching cache entries.
- `make release VERSION=0.1.0` passed the full gate and packaging at `31330c6`; matching core results were cached and packaging tests reran. `python3 /tmp/yoinkdl-stable-smoke.py bin/releases/0.1.0` passed checksums, exact Git source snapshot, licenses/metadata, CGO-disabled settings, isolated installation, HTTP/HTTPS transfers, and certificate refusal. Temporary report: `/tmp/yoinkdl-stable-verification-k4ml0ntq/verification.json`.
- The workflow's exact tag/changelog guard accepted `v0.1.0` and selected only the eight stable entries and scope notes. The candidate notes are unchanged. `gh run view` confirmed stable CI/draft success, and downloaded draft metadata matched stable flags, tagged notes, all three asset sizes/digests, and Go 1.27.0 build metadata.
- `gh release download v0.1.0 --repo devkcud/ydl-re --dir bin/releases/0.1.0-hosted` succeeded. `python3 /tmp/yoinkdl-stable-smoke.py bin/releases/0.1.0-hosted` passed the same installation, byte-identity, HTTP/HTTPS, and refusal checks. Temporary report: `/tmp/yoinkdl-stable-verification-rddqntk9/verification.json`. Checks involving localhost ran outside the sandbox; durable results are in the release guide.
- `python3 /tmp/yoinkdl-doccheck.py` passed: 24 Markdown files, 290 local links/anchors, 32 shell blocks, five JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. Original-design `<id>` shell placeholders were normalized for syntax checking; its intentional hard-break spaces remain unchanged.
- Stable public-download verification awaits publication. Native foreign-platform execution, full-disk/network-filesystem/power-loss qualification, and interrupted remote upload/retry behavior remain unverified. These limits do not broaden with cross-compilation or a stable version label.

## Next steps

1. Publish the verified `0.1.0` stable draft when ready, preserving `v0.1.0` at `31330c6` and all three verified assets. Check unauthenticated release/download access, exact checksums, and installed HTTP/HTTPS behavior, then refresh this handoff. Preserve `0.1.0-rc.1` as a separate prerelease.
2. After stable publication, address actual reported defects before optional roadmap work. The next feature milestone is v0.2's daemon and persistent queue; settle IPC, persistence, scheduling, and shared bandwidth contracts before implementation. Broader native support requires separate runtime qualification.

## Open decisions and blockers

The stable draft, assets, local/hosted gates, and installed binary are verified. Publication is the remaining v0.1 release step. No reported regression awaits reproduction. Broader native qualification, durability, signing, and package-manager distribution remain outside current evidence.
