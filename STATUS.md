# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E5 are complete locally. Repository builds expose the explicit Linux daemon/queue CLI alongside independent standalone downloads.

E6 crash fixtures are complete locally: saved settings/FIFO order, active checkpoints, explicit resume, pending pause/cancel intents, and death after verified publication before queue completion. Capacity, broader process concurrency/shared-rate behavior, and final release gates remain unfinished. No v0.2 release or automatic daemon startup was added.

## Handoff

- `cmd/yoinkdl/daemon_crash_linux_test.go` kills the daemon after two active jobs receive durable prefixes, with three more jobs waiting. It verifies saved running intents, sources, order, and concurrency before restart; restart reuses ownership lock inodes, replaces the stale endpoint, and admits waiting jobs in FIFO order. Interrupted attempts stay idle even with free capacity. Explicit resume joins the tail, validates the saved ETag/suffix, publishes exact fixture hashes, and removes terminal sources/sidecars.
- `internal/daemon/crash_linux_test.go` uses the existing private `newServer` runner seam in child test processes. It holds a real `Budget.Download` result after successful publication or cancellation cleanup, then kills before the coordinator receives it. Fresh normal runners recover running/pausing/canceling as interrupted/paused/canceled with unknown publication and the expected source retention. No production code or hooks changed.
- Published outputs retain exact bytes and their inode across restart, refused retry, cancellation, and forget. An attempted resume against an existing final file fails without HTTP traffic or overwrite; canceled records refuse resume. Paused prefixes resume with the saved validator, while canceled prefixes/manifests survive metadata-only forget unchanged. The fixtures cover process death after storage cleanup, not power loss or every storage syscall boundary.
- E5 CLI remains in `internal/cli`: shared redacted options, caller-relative output resolution, random 128-bit job IDs, safe text/schema-v1 JSON, and single-attempt IPC. Unknown mutation outcomes retain the submitted ID; failed client output cannot cancel accepted work. `cmd/yoinkdl/daemon_linux_test.go` covers clean restart, signals, conditional resume, failed output, and startup cleanup.
- `internal/daemon` owns private Linux roots/locks, anchored sockets, peer-UID checks, bounded protocol/clients, and joined shutdown. `New` prepares without dispatch; `Run` executes once; `Close` bypasses the control mailbox. Non-Linux operations explicitly fail.
- Preserve E1–E3 rules: durable intents before effects, FIFO admission, concurrency `1..8`, slots retained through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget across job/control changes.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), and [testing evidence](docs/testing.md) own the detailed contracts. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- Focused tests passed: `go test ./cmd/yoinkdl -run '^TestDaemonProcessCrashPreservesQueueAndResume$' -count=1` and `go test ./internal/daemon -run '^TestDaemonProcessCrash(AfterPublication|DuringControlCleanup)$' -count=1`. The first sandbox attempt failed before testing because the Go cache was read-only; approved runs outside the sandbox passed with local HTTP/Unix sockets and real subprocesses.
- `make check` passed after each crash slice: formatting, vet, ordinary/race tests, and build on Go `1.27.2-X:nodwarf5` Linux/amd64. The first gate executed every package; the second reran changed daemon and packaging suites and reused matching cached results elsewhere. No product test failed.
- `CGO_ENABLED=0 go test ./cmd/yoinkdl ./internal/daemon -run '^TestDaemonProcessCrash' -count=1` passed all new crash fixtures.
- `python3 /tmp/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 348 local links/anchors, 34 shell blocks, seven JSON and one TOML example, fences, and whitespace. The unchanged original design's illustrative commands are excluded. `git diff --check` passed, including new test code once staged.
- E5's six-target CLI/test compilation remains historical compile-only evidence. Final E6 whole-suite CGO-disabled and six-target gates wait for the remaining capacity/concurrency/rate fixtures. Real power-loss durability and native platforms beyond Linux/amd64 remain unverified.

## Next steps

1. Qualify retained queue capacity through a real daemon process: fill all 256 records including terminal jobs, restart, prove admission stays refused until forget, and verify released capacity/output reservations without deleting final or partial artifacts.
2. Qualify whole-process concurrency/shared-rate behavior, including live increases/reductions, restart, and cancellation. Keep these tests distinct from in-process coordinator/budget evidence; never reset shared credit during live controls.
3. Run final native ordinary/race/CGO-disabled process gates and six-target compilation, refresh security/platform/release documentation and this handoff, and split local commits. Prepare v0.2 release evidence locally; preserve existing tags/assets and never push.

## Open decisions and blockers

No product blocker is known. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E6 capacity, broader concurrency/rate, and final release qualification remain unfinished. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
