# Project status

Updated: October 12, 2026.

## Current state

Milestones A, B, and C are implemented. Milestone D includes aggregate rate caps, versioned JSON/text output, the local security review, native Linux/amd64 evidence, and CI/release tooling. The full gate, uncached CGO-disabled tests, and six-target compilation now pass in hosted Ubuntu CI with the minimum Go 1.27.0 toolchain.

The owner selected MPL 2.0 without a named copyright holder, Linux/amd64 GitHub Releases, public GitHub Issues reporting, and `0.1.0-rc.1` for the first candidate. Version-specific notes and local candidate archives are the next work. Remote draft creation and publication have not run; a full v0.1 release is not complete.

## Handoff

- [CI run 38294238535](https://github.com/devkcud/ydl-re/actions/runs/38294238535) passed for `1ce24810b7b6a05607c30ab37ff725758e29ebb8`, matching remote main and the checkout before these documentation updates. Both jobs report Ubuntu 24.04.5 LTS, runner image `20261004.327.1`, Go 1.27.0, `GOTOOLCHAIN=local`, and no Go experiments. [Platforms](docs/platforms.md#hosted-ci-evidence) records the commands and evidence limits. No runner fixes were required.
- [CI and releases](docs/releases.md) owns version/tag syntax, clean-snapshot packaging, changelog headings, draft creation, and installation. `make release VERSION=X.Y.Z` requires a clean committed tree, runs the full gate, and creates binary/source archives plus SHA256SUMS under ignored `bin/releases/<version>/`. Existing output is preserved; packaging does not tag or publish.
- `.github/workflows/release.yml` handles pushed `v*` tags and requires an exact nonempty version section in `CHANGELOG.md`. It creates a draft only and refuses existing releases; an interrupted upload may leave an incomplete draft. GitHub currently lists no releases. `CHANGELOG.md` remains Unreleased until candidate preparation.
- Packaging tests cover real archives, source/checksum identity, extracted-binary downloads, repeat-build equality, refusal, and failure cleanup. Their runtime checks require native Linux/amd64 with Bash, Git, GNU tar/coreutils, and gzip. The committed Go runtime notice avoids dependence on the local `$GOROOT/LICENSE` layout.
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directory, private single-link mutable partials, strict representation validation, atomic checkpoints, offline verified-ready recovery, explicit overwrite, and stable lock inodes. Output failures retain verified files.
- Native Linux/arm64/macOS remain unqualified; Windows storage is unsupported. Hosted compilation does not broaden runtime support or establish filesystem-specific durability. The original design remains unchanged. Keep focused local commits and never push.

## Validation

- `gh run view 38294238535 --repo devkcud/ydl-re --json headSha,conclusion,status,url,jobs` and `--log` confirmed both jobs and all steps succeeded. The logs show `make check`, `CGO_ENABLED=0 go test -count=1 ./...`, and the six-target CLI/test compilation loop passed. Ordinary, race, and CGO-disabled runs include all six packages, including `tools`.
- `gh api repos/devkcud/ydl-re/commits/main` confirmed the tested revision; `gh release list --repo devkcud/ydl-re` returned no releases. Sandboxed GitHub access failed; read-only queries outside the sandbox succeeded. No remote writes were performed.
- This step changes documentation only. Matching hosted/runtime results were reused; no local runtime gate was repeated. `python3 /tmp/yoinkdl-doccheck.py` passed: 24 Markdown files, 269 local links/anchors, 32 shell blocks, five JSON examples, two TOML examples, fences, and whitespace. One original-design shell block needed its illustrative `<id>` placeholders normalized for syntax checking; the original file and its intentional hard-break spaces remain unchanged. `git diff --check` passed.
- Prior local evidence remains in [testing](docs/testing.md) and [platforms](docs/platforms.md): Go 1.27.2 full gate, ext4/tmpfs checks, packaging regressions, workflow linting, and six-target compilation. Native foreign-platform execution, full-disk/network-filesystem/power-loss qualification, and remote draft/upload behavior remain unverified.

## Next steps

1. Prepare the owner-selected `0.1.0-rc.1` changelog section, commit it, then build local candidate archives from that clean revision. Verify checksums, source revision, version output, and installation/download behavior; record exact evidence here.
2. Have the maintainer push the prepared commits and create/push `v0.1.0-rc.1` at the intended release revision. Inspect the generated draft, all three assets, matching sources, and installation before publication. Never push automatically.

## Open decisions and blockers

Hosted CI and the first candidate version are settled. Candidate packaging remains local work; draft creation requires the maintainer's remote actions. No stable release exists. Broader native qualification, durability, signing, and package-manager distribution remain outside current evidence. Configuration files and daemon/TUI work stay optional/later.
