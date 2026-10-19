# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E5 are complete locally. Repository builds expose the explicit Linux daemon/queue CLI alongside independent standalone downloads.

E6 crash, retained-capacity, and process concurrency fixtures are complete locally. Shared-rate process qualification and final release gates remain unfinished. No v0.2 release or automatic daemon startup was added.

## Handoff

- `cmd/yoinkdl/daemon_parallel_linux_test.go` gates HTTP responses in a real CLI daemon: default sequential admission, live increase to eight jobs, non-preemptive reduction to two, FIFO replacement after cancellation, and persisted reduced concurrency after clean restart. Paused jobs remain paused; outputs and retained prefixes match exact fixture bytes.
- `internal/daemon/drain_linux_test.go` holds actual canceled transfer results through the existing private runner seam in a child process. Pausing/canceling jobs retain their active slot and refuse forget until the wrapper returns; only then can the waiting job start. `crash_linux_test.go` shares its subprocess launcher with this fixture and adds an input pipe for release. No production code changed.
- `cmd/yoinkdl/daemon_capacity_linux_test.go` covers 256 retained terminal jobs, full-queue refusal/deduplication, and durable forget across forced/clean restarts. Released slots get fresh admission orders; existing final files remain protected and canceled prefixes resume conditionally.
- Earlier crash fixtures remain in `cmd/yoinkdl/daemon_crash_linux_test.go` and `internal/daemon/crash_linux_test.go`: FIFO/settings and lock recovery, explicit checkpoint resume, pending controls, and unknown publication after process death. The private runner seam holds actual transfer results only in test code; no production hook was added. These tests do not prove power-loss durability or every storage syscall boundary.
- E5 CLI remains in `internal/cli`: shared redacted options, caller-relative output resolution, random 128-bit job IDs, safe text/schema-v1 JSON, and single-attempt IPC. Unknown mutation outcomes retain the submitted ID; failed client output cannot cancel accepted work. `cmd/yoinkdl/daemon_linux_test.go` covers clean restart, signals, conditional resume, failed output, and startup cleanup.
- `internal/daemon` owns private Linux roots/locks, anchored sockets, peer-UID checks, bounded protocol/clients, and joined shutdown. `New` prepares without dispatch; `Run` executes once; `Close` bypasses the control mailbox. Non-Linux operations explicitly fail.
- Preserve E1–E3 rules: durable intents before effects, FIFO admission, concurrency `1..8`, slots retained through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget across job/control changes.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), and [testing evidence](docs/testing.md) own the detailed contracts. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- `go test ./cmd/yoinkdl ./internal/daemon -run '^TestDaemonProcess(ParallelControls|RetainsDrainingSlot|Crash)' -count=1` passed, including the affected crash helper regressions.
- `CGO_ENABLED=0 go test ./cmd/yoinkdl ./internal/daemon -run '^TestDaemonProcess(ParallelControls|RetainsDrainingSlot)' -count=1` passed.
- `make check` passed: formatting, vet, ordinary/race tests, and build on Go `1.27.2-X:nodwarf5` Linux/amd64. CLI/process, daemon, and packaging suites executed; unchanged packages reused matching cached results. Approved execution outside the sandbox allowed Go cache writes and local HTTP/Unix sockets. No product test failed.
- Earlier crash/capacity slices passed focused ordinary/race/CGO-disabled checks. E5's six-target compilation remains historical compile-only evidence. Final E6 whole-suite CGO-disabled and six-target gates wait for the remaining rate fixtures.
- `python3 /tmp/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 351 local links/anchors, 34 shell blocks, seven JSON and one TOML example, fences, and whitespace. `git diff --check` passed, including new tests once staged. The unchanged original design's illustrative commands are excluded.

## Next steps

1. Qualify whole-process shared-rate behavior: composed private/shared caps across simultaneous jobs, pause/resume, cancellation, completion, live parallel controls, and restart. Keep this evidence distinct from in-process budget tests; never reset shared credit during live controls.
2. Run final native ordinary/race/CGO-disabled process gates and six-target compilation, refresh security/platform/release documentation and this handoff, and split local commits. Prepare v0.2 release evidence locally; preserve existing tags/assets and never push.

## Open decisions and blockers

No product blocker is known. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E6 shared-rate process and final release qualification remain unfinished. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
