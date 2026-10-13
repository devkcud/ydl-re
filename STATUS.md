# Project status

Updated: October 12, 2026 (America/Sao_Paulo).

## Current state

Milestones A, B, and C are implemented. Milestone D includes aggregate rate caps, versioned JSON/text output, the local security review, native Linux/amd64 evidence, and passing hosted CI/release tooling. The full gate, uncached CGO-disabled tests, and six-target compilation pass with the minimum Go 1.27.0 toolchain.

The owner-selected `0.1.0-rc.1` now has a remote tag and a verified GitHub prerelease draft with binary/source archives and checksums. The draft is unpublished; a stable v0.1 release is not complete. MPL 2.0, Linux/amd64 GitHub Releases, and public GitHub Issues reporting remain the selected policies.

## Handoff

- The owner explicitly authorized this release push. An atomic push advanced main to `8a6d5b95de472f42a2f043e628fc5f8788470e89` and created annotated tag `v0.1.0-rc.1` at `5fae374cd31429f5d8653c6c497dcf0d21865c7e`. Preserve that tag identity. Evidence-only documentation commits after the tag are outside the source archives.
- [Draft run 38335647469](https://github.com/devkcud/ydl-re/actions/runs/38335647469) passed all steps and created the [YoinkDL 0.1.0-rc.1 draft](https://github.com/devkcud/ydl-re/releases/tag/untagged-62d19276c4de45dd188a). Its prerelease flag, exact changelog body, all three uploaded assets, checksums, source revision, and isolated installation/download behavior are verified. Publication remains pending.
- [CI run 38335647491](https://github.com/devkcud/ydl-re/actions/runs/38335647491) passed both jobs at the pushed main revision. The release runner used Ubuntu 24.04.5, image `20261004.327.1`, Go 1.27.0, and no Go experiments. [Platform evidence](docs/platforms.md) retains the original hosted/native qualification details.
- [Hosted candidate verification](docs/releases.md#hosted-candidate-verification) records durable hashes and results. Downloaded assets are under ignored `bin/releases/0.1.0-rc.1-hosted/`; the earlier local Go 1.27.2 build remains under `bin/releases/0.1.0-rc.1/`. Their source archive is identical; the binary archives differ with the toolchain. Keep the checksum file paired with its own archives.
- [CI and releases](docs/releases.md) owns clean-snapshot packaging, exact changelog headings, draft creation, and installation. Existing release/output state is preserved; interrupted uploads may leave incomplete drafts and the workflow refuses to overwrite them. No application, workflow, or repository-setting changes were needed.
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directory, private single-link mutable partials, strict representation validation, atomic checkpoints, offline verified-ready recovery, explicit overwrite, and stable lock inodes. Output failures retain verified files.
- Linux/arm64/macOS remain unqualified natively; Windows storage is unsupported. Hosted compilation adds no filesystem-specific durability guarantees. The original design remains unchanged. This release task has explicit push authorization; the default no-push policy still applies to unrelated work.

## Validation

- `git ls-remote origin refs/heads/main refs/tags/v0.1.0-rc.1 'refs/tags/v0.1.0-rc.1^{}'` verified the pushed main revision and tag target. `gh run view` confirmed successful runs `38335647491` and `38335647469`; the release log records the full gate, ordinary/race tests for all six packages, packaging, checksum validation, and upload completion.
- `gh release view v0.1.0-rc.1 --repo devkcud/ydl-re --json tagName,name,isDraft,isPrerelease,publishedAt,targetCommitish,url,body,assets` confirmed an unpublished prerelease draft. Exact release notes, uploaded states, all three sizes, and GitHub SHA-256 digests matched the downloaded files. The actual tag target was verified separately from `targetCommitish`.
- `gh release download v0.1.0-rc.1 --repo devkcud/ydl-re --dir bin/releases/0.1.0-rc.1-hosted` succeeded. `python3 /tmp/yoinkdl-hosted-release-smoke.py bin/releases/0.1.0-rc.1-hosted` passed outside the sandbox: checksums, exact Git source snapshot, licenses/metadata, Go 1.27.0/CGO-disabled settings, isolated installation, version/help, and one-/four-connection downloads of 9,437,201 bytes with exact hashes and successful JSON. Temporary detail is `/tmp/yoinkdl-hosted-rc1-verification-l90avvgd/verification.json`; durable results are in the release guide.
- This step changes documentation only. Hosted release/runtime results were reused; no redundant local full gate was run. `python3 /tmp/yoinkdl-doccheck.py` passed: 24 Markdown files, 278 local links/anchors, 32 shell blocks, five JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. The original design's illustrative `<id>` shell placeholders were normalized for syntax checking; its intentional hard-break spaces remain unchanged.
- Public download behavior remains unverified until publication. Native foreign-platform execution, full-disk/network-filesystem/power-loss qualification, and interrupted remote upload/retry behavior remain unverified. Broader evidence and reproduction commands remain in [testing](docs/testing.md), [platforms](docs/platforms.md), and [releases](docs/releases.md).

## Next steps

1. Publish the verified `0.1.0-rc.1` prerelease draft when ready to make it public, preserving the tag and verified assets. Then verify unauthenticated release/download access, checksums, and installation from the public release, and refresh this handoff.
2. Collect candidate feedback and qualify any fixes before preparing a stable `0.1.0`. Broader native support requires separate runtime evidence; configuration files and daemon/TUI work stay optional/later.

## Open decisions and blockers

The tag, hosted draft workflow, uploaded assets, and installed candidate are verified. The draft remains unpublished and no stable release exists. Broader native qualification, durability, signing, and package-manager distribution remain outside current evidence.
