# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E6 are complete locally. Repository builds expose the explicit Linux daemon/queue CLI alongside independent standalone downloads.

Local `0.2.0-rc.1` archives and installed-binary verification pass. No v0.2 tag, hosted run, upload, or publication was performed. Hosted release verification remains separate work.

## Handoff

- The immutable candidate source is `34041c372519ac751bb9d6ec1ecda56184d13161`, containing the exact `CHANGELOG.md` candidate section. Archives and `SHA256SUMS` are under ignored `bin/releases/0.2.0-rc.1/`; [release evidence](docs/releases.md#v02-local-qualification) records both archive hashes. Later documentation commits are not the candidate source revision.
- Runtime/test/build sources are unchanged from clean E6 qualification revision `5c00f2e1e522268c27ab89aba7d1f479fa12950f`. This continuation changed documentation only. Native/platform gates, security coverage, installed candidate checks, and support limits are recorded in the owning guides.
- E6 CLI process fixtures cover forced death, checkpoint/FIFO recovery, capacity/forget, live concurrency, and composed shared/private rates. `internal/daemon/crash_linux_test.go` and `drain_linux_test.go` use the existing private runner seam at publication/control boundaries. No production hook exists; these fixtures do not prove power-loss durability or every storage syscall boundary.
- Preserve queue durability before effects, FIFO admission, concurrency `1..8`, slots through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget. Queued transfers never overwrite final files. Clients never start daemons implicitly or retry unknown mutations; failed client output cannot cancel accepted work.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), [testing](docs/testing.md), [platforms](docs/platforms.md), and [security](docs/security.md) own the detailed contracts. Preserve `v0.1.0` at `31330c6` and `v0.1.0-rc.1` at `5fae374`, plus their existing assets. Keep focused local commits; never push.

## Validation

- `TMPDIR="$PWD/bin/t.MBOfTk" make check` passed on ext4: formatting, vet, ordinary tests, race tests, and build. All seven filesystem/process packages reran in both modes; pure range planning reused matching cached results.
- `CGO_ENABLED=0 go test -count=1 ./...` passed all eight packages uncached on tmpfs. Native execution used Arch Linux/amd64, kernel `7.2.11-zen1-1-zen`, Go `1.27.2-X:nodwarf5`, `GOAMD64=v1`, and GCC `16.2.1 20260810` for race checks.
- `bash /tmp/yoinkdl-e6-matrix.sh "$PWD/bin/e6-qualification.0Stkvu/matrix"` passed the documented six-target CLI/test compilation recipe with CGO disabled. Each target produced eight test programs; build metadata records `5c00f2e` and `vcs.modified=false`. ELF/Mach-O/PE architectures were inspected. Only the native CLI version command ran from these artifacts; foreign binaries did not execute.
- `TMPDIR="$PWD/bin/t.MBOfTk" make release VERSION=0.2.0-rc.1` passed from clean `34041c3`, rerunning the filesystem/process suites ordinarily and with the race detector before packaging. `python3 bin/e6-qualification.0Stkvu/yoinkdl-verify-release.py 0.2.0-rc.1` passed archive checksums, exact source export, licenses/metadata, isolated installation, HTTP/HTTPS one-/four-worker downloads, and untrusted-certificate refusal.
- The installed daemon passed private source retention/clearing, safe views, explicit startup, pause, clean restart, persisted concurrency, conditional resume from 131,072 bytes, protected final-file refusal, cancellation, metadata-only forget, and clean stop. Outputs matched the 9,437,201-byte fixture hash in the release report. Existing tag objects and all 18 existing v0.1 archive/checksum files were verified unchanged.
- Evidence and rerunnable scripts are retained under ignored `bin/e6-qualification.0Stkvu/`, including `native-ext4.log`, `cgo-disabled.log`, `matrix-host.log`, `environment.json`, `release.log`, and `installed-candidate.json`. An initial sandbox attempt blocked listeners; a long temporary path exceeded the socket limit. A shorter ext4 path passed outside the sandbox. Sandboxed macOS package resolution failed despite present sources; the same full recipe passed outside it.
- `python3 /tmp/yoinkdl-doccheck.py`, the workflow's exact-heading candidate-note extraction, `bash -n tools/release.sh`, and `git diff --check` passed. Documentation checks cover local links/anchors, shell/JSON/TOML examples, fences, and whitespace; the original design's illustrative commands remain excluded.
- Hosted v0.2 CI, foreign native execution, and real power loss were not tested. No remote state changed.

## Next steps

1. For a later explicitly requested release, recheck the local candidate checksums and source revision above, then qualify hosted Go 1.27.0 execution and downloaded assets before publication. Keep the candidate local under the current no-push rule; do not substitute the later documentation HEAD for its source revision.
2. After release work, define the first v0.3 slice from the [roadmap](docs/roadmap.md#v03--tui-and-richer-local-workflows) before implementing a TUI, profiles, history, or optional hooks. No local E6 implementation or qualification remains.

## Open decisions and blockers

No local E6 product blocker is known. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. Hosted v0.2 verification/publication remains outside the current local scope. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
