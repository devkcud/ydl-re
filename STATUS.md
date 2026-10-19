# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E5 are complete locally. Repository builds expose the explicit Linux daemon/queue CLI alongside independent standalone downloads.

E6 crash and retained-capacity fixtures are complete locally: conservative recovery, explicit resume, pending controls, publication ambiguity, and full-queue/forget recovery across process restarts. Broader process concurrency/shared-rate behavior and final release gates remain unfinished. No v0.2 release or automatic daemon startup was added.

## Handoff

- `cmd/yoinkdl/daemon_capacity_linux_test.go` fills all 256 records through IPC to an actual CLI daemon process. Completed/canceled jobs retain capacity, deduplication, and output reservations across forced death. Full-queue refusals and absent-ID forget leave the snapshot/order unchanged; CLI JSON reports a redacted `capacity` failure without creating output artifacts. Active jobs refuse forget.
- Forget durably frees one slot and its output reservation across forced and clean restart. Reusing the forgotten ID assigns a new order: existing final output fails without another HTTP request or inode/content changes, while a preserved canceled prefix resumes conditionally to exact bytes. Refilling capacity restores refusal. No production code changed.
- Earlier crash fixtures remain in `cmd/yoinkdl/daemon_crash_linux_test.go` and `internal/daemon/crash_linux_test.go`: FIFO/settings and lock recovery, explicit checkpoint resume, pending controls, and unknown publication after process death. The private runner seam holds actual transfer results only in test code; no production hook was added. These tests do not prove power-loss durability or every storage syscall boundary.
- E5 CLI remains in `internal/cli`: shared redacted options, caller-relative output resolution, random 128-bit job IDs, safe text/schema-v1 JSON, and single-attempt IPC. Unknown mutation outcomes retain the submitted ID; failed client output cannot cancel accepted work. `cmd/yoinkdl/daemon_linux_test.go` covers clean restart, signals, conditional resume, failed output, and startup cleanup.
- `internal/daemon` owns private Linux roots/locks, anchored sockets, peer-UID checks, bounded protocol/clients, and joined shutdown. `New` prepares without dispatch; `Run` executes once; `Close` bypasses the control mailbox. Non-Linux operations explicitly fail.
- Preserve E1–E3 rules: durable intents before effects, FIFO admission, concurrency `1..8`, slots retained through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget across job/control changes.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), and [testing evidence](docs/testing.md) own the detailed contracts. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- `go test ./cmd/yoinkdl -run '^TestDaemonProcessCapacityAndForgetRecovery$' -count=1` passed, as did `CGO_ENABLED=0 go test ./cmd/yoinkdl -run '^TestDaemonProcessCapacityAndForgetRecovery$' -count=1`.
- `make check` passed: formatting, vet, ordinary/race tests, and build on Go `1.27.2-X:nodwarf5` Linux/amd64. Changed CLI/process and packaging suites executed; unchanged packages reused matching cached results. Approved execution outside the sandbox allowed Go cache writes and local HTTP/Unix sockets. No product test failed.
- Earlier crash slices passed focused ordinary/race/CGO-disabled checks. E5's six-target CLI/test compilation remains historical compile-only evidence. Final E6 whole-suite CGO-disabled and six-target gates wait for the remaining concurrency/rate fixtures.
- `python3 /tmp/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 349 local links/anchors, 34 shell blocks, seven JSON and one TOML example, fences, and whitespace. `git diff --check` passed, including the new test once staged. The unchanged original design's illustrative commands are excluded. Real power-loss durability and native platforms beyond Linux/amd64 remain unverified.

## Next steps

1. Qualify whole-process concurrency/shared-rate behavior. Start with server-gated live parallel increases/reductions and slot retention during cancellation, then verify composed caps across jobs, controls, and restart. Keep this evidence distinct from in-process coordinator/budget tests; never reset shared credit during live controls.
2. Run final native ordinary/race/CGO-disabled process gates and six-target compilation, refresh security/platform/release documentation and this handoff, and split local commits. Prepare v0.2 release evidence locally; preserve existing tags/assets and never push.

## Open decisions and blockers

No product blocker is known. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E6 broader process concurrency/rate and final release qualification remain unfinished. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
