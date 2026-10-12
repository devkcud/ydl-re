# Project status

Updated: October 12, 2026.

## Current state

Milestones A, B, and C are implemented and pass the local Linux gate. The CLI supports sequential/parallel transfers, safe resume, exclusive offset writes, verified checkpoints, bounded retries, inspection, cancellation, overwrite protection, and inactivity timeouts. Milestone D includes aggregate body-rate caps through `--limit` and versioned newline-delimited output through `--json`.

Text/JSON output, the local security review, platform evidence, and local CI/release tooling are implemented. The owner selected MPL 2.0 without a named copyright holder, GitHub Releases for initial Linux/amd64 binaries, and public GitHub Issues reporting. Hosted Actions, remote draft creation, and publication have not run. Native qualification beyond Linux/amd64 remains open. A full v0.1 release is not complete.

## Handoff

- [CI and releases](docs/releases.md) owns the packaging, version/tag, changelog, draft, and installation contracts. `LICENSE` contains Mozilla's official MPL 2.0 text; `LICENSES/Go-BSD-3-Clause.txt` preserves Go's runtime notice. `SECURITY.md` identifies public GitHub Issues; API reads confirmed issues enabled, private reporting disabled, and no existing releases. No remote settings changed.
- `.github/workflows/ci.yml` defines PR/main/manual checks on Ubuntu 24.04: `make check`, uncached CGO-disabled tests, and six-target CLI/test compilation. Checkout v7.0.1 and setup-go v7.0.0 use full verified commit pins, read-only CI permissions, no persisted credentials, and no dependency cache. Go comes from `go.mod`; local replay used Go 1.27.2, so hosted/minimum-patch results remain unverified.
- `make release VERSION=X.Y.Z` runs the full gate, then `tools/release.sh`. Native Linux/amd64 packaging needs Bash, Git, GNU tar/coreutils, and gzip. It requires a clean committed tree, exports a source snapshot, builds CGO-disabled from that snapshot, and verifies the version/help. It emits normalized binary/source archives plus verified SHA256SUMS under ignored `bin/releases/<version>/`. Existing output is preserved. No tag or remote write occurs.
- `tools/release_test.go` builds real archives in temporary Git fixtures, verifies source/checksum identity, runs an extracted binary against a local HTTP server, checks repeated-build equality, and covers invalid/dirty/existing-output refusal and build/smoke-failure cleanup. Packaging tests skip outside native Linux/amd64. Go formatting includes `tools/`.
- `.github/workflows/release.yml` handles pushed `v*` tags. Stable versions are X.Y.Z; alpha/beta/rc prereleases require a positive numeric suffix. An exact nonempty version section in `CHANGELOG.md` is mandatory. The workflow checks/packages and creates a draft only, with all three assets; existing releases are not overwritten. A failed upload may leave an incomplete draft. CHANGELOG remains Unreleased; no release/version tag was created.
- [Platform evidence](docs/platforms.md) retains the native Linux/amd64 ext4/tmpfs baseline and distinguishes compilation from execution. Linux/arm64/macOS storage is enabled but unqualified natively; Windows download/inspection storage is unsupported. The new tooling does not broaden runtime support.
- Preserve [security](docs/security.md) and [storage](docs/resume-and-storage.md) contracts: trusted output directory, private single-link mutable partials, strict representation validation, atomic checkpoints, offline verified-ready recovery, explicit overwrite, and stable lock inodes. Sequential publication adds readback; HTTP connection reuse remains disabled for diagnostic privacy. Output failures retain verified files.
- The original design is unchanged. Keep focused local commits, refresh this handoff after each implementation step, and never push.

## Validation

- Packaging initially failed because this distro omits `$GOROOT/LICENSE`. The script now bundles the official Go notice from the committed snapshot, avoiding installation-layout assumptions. `go test ./tools -count=1` and `go test -race ./tools -count=1` pass, including source/byte checks, a real extracted-binary download, reproducible archives, refusal, and build/smoke-failure cleanup.
- The extracted CI native commands (`bash /tmp/yoinkdl-ci-native.sh`) passed: the full gate plus `CGO_ENABLED=0 go test -count=1 ./...`. After the cleanup and unsuccessful-version regressions, a final `make check` passed formatting, vet, all tests, race tests, and build. Existing core suites used valid cached results on this last run; tooling tests reran.
- `RUNNER_TEMP=/tmp/yoinkdl-release-ci-compile bash /tmp/yoinkdl-ci-compile.sh` passed for all six targets. The final replay after the last tooling-test addition also passed. Foreign tests compile without execution; packaging runtime tests are Linux/amd64-only.
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck= -pyflakes= .github/workflows/ci.yml .github/workflows/release.yml` passed. Bash syntax and extracted tag/changelog guards passed for valid, malformed, injection-shaped, and missing-note inputs. ShellCheck/Pyflakes were not installed, so their optional integrations were disabled. No hosted workflow was run.
- `python3 /tmp/yoinkdl-security-doccheck.py` passed: 24 Markdown files, 263 local links/anchors, 30 shell blocks, four JSON examples, one TOML example, fences, and whitespace. `git diff --check` passed. No core downloader/runtime behavior changed. Native ARM/macOS/Windows execution, hosted Ubuntu/minimum-Go validation, full-disk/network-filesystem/power-loss qualification, and remote upload failure behavior remain unverified.

## Next steps

1. Record the first hosted CI result after the maintainer pushes the prepared commits. Validate the Ubuntu 24.04 runner and the Go patch selected from `go.mod`; address real runner failures before claiming hosted qualification. Do not push automatically.
2. Choose the first release version, move the relevant Unreleased changelog entries under its exact version heading, then have the maintainer create/push the matching tag. Inspect the generated draft, checksums, matching source revision, and installation before publication. Native support beyond Linux/amd64 remains separate work.

## Open decisions and blockers

License, initial channel/target, and reporting choices are settled. Local tooling is validated; hosted CI/draft creation requires the maintainer's remote actions and has not occurred. No stable release exists. Broader native qualification, durability, signing, and package-manager distribution remain outside the current evidence. Configuration files and daemon/TUI work stay optional/later.
