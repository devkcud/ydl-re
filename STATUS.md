# Project status

Updated: October 12, 2026.

## Current state

Milestones A, B, and C are implemented. Milestone D includes aggregate rate caps, versioned JSON/text output, the local security review, native Linux/amd64 evidence, and CI/release tooling. The full gate, uncached CGO-disabled tests, and six-target compilation now pass in hosted Ubuntu CI with the minimum Go 1.27.0 toolchain.

The owner selected MPL 2.0 without a named copyright holder, Linux/amd64 GitHub Releases, public GitHub Issues reporting, and `0.1.0-rc.1` for the first candidate. Version-specific notes and verified local binary/source archives are ready. Remote draft creation and publication have not run; a full v0.1 release is not complete.

## Handoff

- [CI run 38294238535](https://github.com/devkcud/ydl-re/actions/runs/38294238535) passed for `1ce24810b7b6a05607c30ab37ff725758e29ebb8`, matching remote main and the checkout before these documentation updates. Both jobs report Ubuntu 24.04.5 LTS, runner image `20261004.327.1`, Go 1.27.0, `GOTOOLCHAIN=local`, and no Go experiments. [Platforms](docs/platforms.md#hosted-ci-evidence) records the commands and evidence limits. No runner fixes were required.
- [CI and releases](docs/releases.md) owns version/tag syntax, clean-snapshot packaging, changelog headings, draft creation, and installation. `make release VERSION=X.Y.Z` requires a clean committed tree, runs the full gate, and creates binary/source archives plus SHA256SUMS under ignored `bin/releases/<version>/`. Existing output is preserved; packaging does not tag or publish.
- [Candidate verification](docs/releases.md#first-candidate-verification) records archive hashes and local runtime evidence. `bin/releases/0.1.0-rc.1/` contains all three assets from `5fae374cd31429f5d8653c6c497dcf0d21865c7e`; this is the intended tag revision. Later evidence-only documentation commits are outside that source snapshot. Local packaging used Go 1.27.2 with `nodwarf5`; hosted drafts use Go 1.27.0, so inspect their own assets/checksums rather than expecting binary equality.
- `.github/workflows/release.yml` handles pushed `v*` tags and requires an exact nonempty version section in `CHANGELOG.md`. It creates a draft only and refuses existing releases; an interrupted upload may leave an incomplete draft. GitHub currently lists no releases. The seven initial changelog entries now belong to `## [0.1.0-rc.1]`, followed by platform limits; an Unreleased section remains for later work.
- Packaging tests cover real archives, source/checksum identity, extracted-binary downloads, repeat-build equality, refusal, and failure cleanup. Their runtime checks require native Linux/amd64 with Bash, Git, GNU tar/coreutils, and gzip. The committed Go runtime notice avoids dependence on the local `$GOROOT/LICENSE` layout.
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directory, private single-link mutable partials, strict representation validation, atomic checkpoints, offline verified-ready recovery, explicit overwrite, and stable lock inodes. Output failures retain verified files.
- Native Linux/arm64/macOS remain unqualified; Windows storage is unsupported. Hosted compilation does not broaden runtime support or establish filesystem-specific durability. The original design remains unchanged. Keep focused local commits and never push.

## Validation

- `gh run view 38294238535 --repo devkcud/ydl-re --json headSha,conclusion,status,url,jobs` and `--log` confirmed both jobs and all steps succeeded. The logs show `make check`, `CGO_ENABLED=0 go test -count=1 ./...`, and the six-target CLI/test compilation loop passed. Ordinary, race, and CGO-disabled runs include all six packages, including `tools`.
- `gh api repos/devkcud/ydl-re/commits/main` confirmed the tested revision; `gh release list --repo devkcud/ydl-re` returned no releases. Sandboxed GitHub access failed; read-only queries outside the sandbox succeeded. No remote writes were performed.
- `make release VERSION=0.1.0-rc.1` passed formatting, vet, ordinary/race tests, build, packaging, and checksums at `5fae374`. Core tests reused matching cached results; tooling tests reran. The initial sandboxed run failed because `httptest` could not open a localhost listener; the same command passed outside the sandbox. No product or workflow code changed.
- `python3 /tmp/yoinkdl-release-smoke.py` passed outside the sandbox: both checksums, exact `git archive` source identity, licenses/metadata, CGO-disabled build settings, isolated `install -m 0755`, version/help, and one-/four-connection downloads of 9,437,201 bytes with exact hashes and completed/published JSON. Temporary evidence is `/tmp/yoinkdl-rc1-verification-ttv0cm78/verification.json`; durable hashes/results are in the candidate report.
- The exact tag/changelog Bash guard extracted from `.github/workflows/release.yml` accepted `v0.1.0-rc.1` and selected all seven entries plus platform limits, excluding Unreleased. The final `python3 /tmp/yoinkdl-doccheck.py` passed: 24 Markdown files, 273 local links/anchors, 32 shell blocks, five JSON examples, two TOML examples, fences, and whitespace. One original-design shell block needed illustrative `<id>` placeholders normalized for syntax checking; the original and its intentional hard-break spaces remain unchanged. `git diff --check` passed.
- Prior local evidence remains in [testing](docs/testing.md) and [platforms](docs/platforms.md): Go 1.27.2 full gate, ext4/tmpfs checks, packaging regressions, workflow linting, and six-target compilation. Native foreign-platform execution, full-disk/network-filesystem/power-loss qualification, and remote draft/upload behavior remain unverified.

## Next steps

1. Have the maintainer push the prepared commits and create/push `v0.1.0-rc.1` at `5fae374cd31429f5d8653c6c497dcf0d21865c7e`, matching the verified local source archive. Never push automatically. If another release revision is chosen, rebuild and verify it in a separate clean checkout without overwriting these assets.
2. Inspect the tag-triggered draft workflow and all three uploaded assets. Verify its SHA256SUMS, source revision, version/help, and installed download behavior before publication; record the hosted candidate evidence here. Broader native support remains separate work.

## Open decisions and blockers

Hosted CI, candidate notes, and local candidate verification are complete. Draft creation now requires the maintainer's push/tag actions; no stable release exists. Broader native qualification, durability, signing, and package-manager distribution remain outside current evidence. Configuration files and daemon/TUI work stay optional/later.
