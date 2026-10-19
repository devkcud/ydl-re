# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.2.0-rc.1](https://github.com/devkcud/ydl-re/releases/tag/v0.2.0-rc.1) is published as a Linux/amd64 prerelease. Milestones A through D and v0.2 E1 through E6 are complete. Hosted CI, packaging, anonymous downloads, and installed standalone/daemon checks passed. Stable `0.1.0` remains Latest.

The owner explicitly authorized this named prerelease's push and publication after hosted validation. Future implementation remains local under the repository's no-push rule. Stable `0.2.0` promotion has not been requested or performed.

## Handoff

- Preserve candidate source `34041c372519ac751bb9d6ec1ecda56184d13161`, annotated tag object `31bebc5dfc543d318aa47b5ec5fb9c1f69657fa7`, release ID `417837910`, notes, and assets. Publication completed at `2026-10-19T23:07:45Z`. Later documentation commits are not the release source revision.
- The owner had already pushed main at `ae8339a`; only the missing candidate tag was pushed to start draft packaging. Runtime/test/build sources are unchanged from E6 qualification revision `5c00f2e`. This release continuation changes documentation only.
- [Release evidence](docs/releases.md#v02-hosted-qualification-and-publication) records hosted/public hashes, toolchain, checks, and limits. Preserve stable `v0.1.0` at `31330c6` and prerelease `v0.1.0-rc.1` at `5fae374`; both releases retain their original notes and assets, and Latest remains stable `v0.1.0`.
- Local candidate files remain in `bin/releases/0.2.0-rc.1/`; downloaded hosted/public files are in the corresponding `-hosted/` and `-public/` directories. Hosted/public binary hashes differ from the local Go 1.27.2 build. Source archives are byte-identical. Logs, API snapshots, and rerunnable scripts are in ignored `bin/v02-release.8Go8z2/`; earlier native evidence remains in `bin/e6-qualification.0Stkvu/`.
- Preserve queue durability before effects, FIFO admission, concurrency `1..8`, slots through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget. Queued transfers never overwrite final files; clients never implicitly start daemons or retry unknown mutations, and failed client output cannot cancel accepted work.
- E6 boundary fixtures use the existing private runner seam only in tests. They do not prove power-loss durability or every storage syscall boundary. [CLI](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), [testing](docs/testing.md), [platforms](docs/platforms.md), and [security](docs/security.md) retain the detailed contracts.

## Validation

- [CI 39338571399](https://github.com/devkcud/ydl-re/actions/runs/39338571399) passed `make check`, `CGO_ENABLED=0 go test -count=1 ./...`, and six-target CLI/test compilation at pushed main `ae8339a`. [Draft run 39338981764](https://github.com/devkcud/ydl-re/actions/runs/39338981764) passed tag/notes validation, `make release VERSION=0.2.0-rc.1`, and all uploads at tagged source `34041c3`.
- Both hosted runs used Ubuntu 24.04.5, runner image `20261012.334.1`, Go `1.27.0`, `GOTOOLCHAIN=local`, `GOAMD64=v1`, and no Go experiments. Eight-package native ordinary/race and CGO-disabled suites passed. Foreign targets compiled only; hosted filesystem/mount details were not recorded.
- `python3 bin/v02-release.8Go8z2/verify-release.py 0.2.0-rc.1 bin/releases/0.2.0-rc.1-hosted` passed checksums, exact tagged source export, licenses/metadata, installed version/help, HTTP/HTTPS one-/four-worker downloads, untrusted-certificate refusal, private source clearing/safe views, pause/restart/resume, persisted concurrency, protected final-file refusal, cancel/forget preservation, and clean stop. The same command with the `-public` directory passed independently. Outputs matched the 9,437,201-byte fixture; resume used a 131,072-byte prefix.
- `python3 bin/v02-release.8Go8z2/verify-public.py` passed eight anonymous HTTPS requests: candidate page/API/three assets, Latest API, and both prior release APIs. Public assets are byte-identical to the verified draft; asset IDs/sizes/digests and notes are unchanged. Previous releases and Latest were preserved. Remote tag identity was checked before publication.
- The local candidate checksums, exact source identity, unchanged runtime/build sources, both prior local tags, and all 18 prior local release files were rechecked before the authorized tag push. Native ext4/tmpfs evidence from the preceding session remains applicable; no product code changed.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 392 local links/anchors, 36 shell blocks, seven JSON examples, one TOML example, fences, and whitespace. `git diff --check` passed. Product suites were not rerun locally for these documentation-only changes; hosted runtime qualification above covers unchanged product sources.

## Next steps

1. Define the first v0.3 slice from the [roadmap](docs/roadmap.md#v03--tui-and-richer-local-workflows), including user interaction, snapshot/control boundaries, dependency order, and acceptance tests before implementation. Resolve relevant C08/C09/S08 contracts in [open decisions](docs/decisions.md); TUI, profiles, history, and hooks remain unimplemented.
2. Keep all published tags/notes/assets immutable. A stable `0.2.0` promotion needs a separate explicit release request and readiness review; unrelated future implementation remains local/no-push.

## Open decisions and blockers

No v0.2 release blocker remains. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing, package-manager distribution, and v0.3 interaction/configuration contracts remain separate work.
