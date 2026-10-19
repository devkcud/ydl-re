# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E5 are complete locally. Repository builds expose the explicit Linux daemon/queue CLI alongside independent standalone downloads.

E6 qualification is underway. Actual CLI forced-death recovery now covers saved settings/FIFO order, active checkpoints, conservative interruption, and explicit resume to exact output hashes. Publication-boundary, pending pause/cancel intents, capacity, broader process concurrency/rate, and final release gates remain unfinished. No v0.2 release or automatic daemon startup was added.

## Handoff

- `cmd/yoinkdl/daemon_crash_linux_test.go` kills the daemon after two active jobs receive durable prefixes, with three more jobs waiting. It verifies saved running intents, sources, order, and concurrency before restart; restart reuses ownership lock inodes, replaces the stale endpoint, and admits waiting jobs in FIFO order. Interrupted attempts stay idle even with free capacity. Explicit resume joins the tail, validates the saved ETag/suffix, publishes exact fixture hashes, and removes terminal sources/sidecars.
- Next, use the existing private `internal/daemon.newServer` runner seam in a subprocess fixture to hold a real transfer result after verified publication or canceled-transfer cleanup, then kill before queue completion. No production hook is needed. Prove restart reconciles running/pausing/canceling conservatively and preserves final data without inferred success or overwrite.
- E5 CLI remains in `internal/cli`: shared redacted options, caller-relative output resolution, random 128-bit job IDs, safe text/schema-v1 JSON, and single-attempt IPC. Unknown mutation outcomes retain the submitted ID; failed client output cannot cancel accepted work. `cmd/yoinkdl/daemon_linux_test.go` covers clean restart, signals, conditional resume, failed output, and startup cleanup.
- `internal/daemon` owns private Linux roots/locks, anchored sockets, peer-UID checks, bounded protocol/clients, and joined shutdown. `New` prepares without dispatch; `Run` executes once; `Close` bypasses the control mailbox. Non-Linux operations explicitly fail.
- Preserve E1–E3 rules: durable intents before effects, FIFO admission, concurrency `1..8`, slots retained through cleanup, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared body budget across job/control changes.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), and [testing evidence](docs/testing.md) own the detailed contracts. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- `go test ./cmd/yoinkdl -run '^TestDaemonProcessCrashPreservesQueueAndResume$' -count=1` passed on Linux/amd64. The first sandbox run failed before testing because the Go cache was read-only; the approved run outside the sandbox passed with local HTTP/Unix sockets and real subprocesses.
- `make check` passed: formatting, vet, ordinary/race tests, and build on Go `1.27.2-X:nodwarf5` Linux/amd64. All package suites executed successfully; no product test failed.
- `python3 /tmp/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 345 local links/anchors, 34 shell blocks, seven JSON and one TOML example, fences, and whitespace. The unchanged original design's illustrative commands are excluded. `git diff --check` passed.
- E5 previously passed focused CLI/process tests, `make check`, CGO-disabled CLI/process tests, six-target CLI/test compilation, and documentation checks. Those foreign builds were compile-only. This slice has not yet rerun the final E6 CGO-disabled or six-target gates.
- Publication-boundary and pending-control crash fixtures, retained-capacity recovery, broader process concurrency/rate tests, and final release gates are not yet complete. Real power-loss durability and native platforms beyond Linux/amd64 remain unverified.

## Next steps

1. Finish deterministic E6 crash fixtures through the existing daemon runner seam: kill after verified publication before queue completion, and after acknowledged pause/cancel before the runner returns. Verify unknown/interrupted recovery, terminal source clearing, explicit resume, and preserved outputs without overwrite.
2. Qualify retained queue capacity/forget recovery and whole-process concurrency/shared-rate behavior, including restart and cancellation. Keep these tests distinct from in-process coordinator/budget evidence; never reset shared credit during live controls.
3. Run final native ordinary/race/CGO-disabled process gates and six-target compilation, refresh security/platform/release documentation and this handoff, and split local commits. Prepare v0.2 release evidence locally; preserve existing tags/assets and never push.

## Open decisions and blockers

No product blocker is known. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E6 crash-boundary and final release qualification remain unfinished. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
