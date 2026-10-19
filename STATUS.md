# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E5 are complete locally. Repository builds expose the explicit Linux daemon/queue CLI alongside independent standalone downloads.

E6 process fixtures and final native/platform gates pass locally. Candidate archive/installation verification remains unfinished. No v0.2 release or automatic daemon startup was added.

## Handoff

- Final runtime/test qualification used clean revision `5c00f2e1e522268c27ab89aba7d1f479fa12950f`. No production code changed. The platform, security, testing, and release guides now identify the completed gates and remaining candidate work.
- E6 CLI process fixtures cover forced death, checkpoint/FIFO recovery, capacity/forget, live concurrency, and composed shared/private rates. `internal/daemon/crash_linux_test.go` and `drain_linux_test.go` use the existing private runner seam to hold real transfer results at publication/control boundaries. No production hook exists; these fixtures do not prove power-loss durability or every storage syscall boundary.
- Preserve queue durability before effects, FIFO admission, concurrency `1..8`, slots through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget across job/control changes. Initial queued transfers never overwrite final files.
- E5 clients never start a daemon implicitly or retry unknown mutations. Safe text/JSON results retain submitted IDs after lost replies; failed client output cannot cancel accepted work. Linux ownership uses private roots, distinct locks, anchored sockets, peer-UID checks, and bounded IPC with joined shutdown. Non-Linux daemon operations explicitly fail.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), [testing](docs/testing.md), [platforms](docs/platforms.md), and [release qualification](docs/releases.md#v02-local-qualification) own the detailed evidence and contracts. Preserve `v0.1.0` at `31330c6` and `v0.1.0-rc.1` at `5fae374`, plus all existing assets. Keep focused local commits; never push.

## Validation

- `TMPDIR="$PWD/bin/t.MBOfTk" make check` passed on ext4: formatting, vet, ordinary tests, race tests, and build. All seven filesystem/process packages reran in both modes; pure range planning reused matching cached results.
- `CGO_ENABLED=0 go test -count=1 ./...` passed all eight packages uncached on tmpfs. Native execution used Arch Linux/amd64, kernel `7.2.11-zen1-1-zen`, Go `1.27.2-X:nodwarf5`, `GOAMD64=v1`, and GCC `16.2.1 20260810` for race checks.
- `bash /tmp/yoinkdl-e6-matrix.sh "$PWD/bin/e6-qualification.0Stkvu/matrix"` passed the documented six-target CLI/test compilation recipe with CGO disabled. Each target produced eight test programs; build metadata records the checked revision and `vcs.modified=false`. ELF/Mach-O/PE architectures were inspected. Only the native CLI version command ran from these artifacts; foreign binaries did not execute.
- Evidence is under ignored `bin/e6-qualification.0Stkvu/`, including `native-ext4.log`, `cgo-disabled.log`, `matrix-host.log`, and `environment.json`. Initial sandbox listeners were blocked; a longer temporary path exceeded the socket limit. A shorter ext4 path passed outside the sandbox. Sandboxed macOS package resolution also failed despite present standard-library sources; the same complete recipe passed outside it.
- Documentation links/anchors, shell/JSON/TOML examples, fences, and whitespace are checked with `python3 /tmp/yoinkdl-doccheck.py`; the final documentation diff is checked before commit.
- Hosted v0.2 CI, candidate archives, and installed candidate commands have not yet been verified. Foreign native execution and real power loss remain unverified.

## Next steps

1. Prepare `0.2.0-rc.1` notes and local archives from a clean committed tree; verify checksums, exact source identity, licenses/metadata, and isolated standalone/daemon installation. Record candidate-specific evidence and update this handoff.
2. Keep the verified candidate local. Any hosted CI, tag creation, upload, or publication needs a later explicit release instruction; never push under the current repository rules.

## Open decisions and blockers

No product blocker is known. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. Local candidate packaging/installation remains. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
