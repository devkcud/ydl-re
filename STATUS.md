# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) remains the published stable Linux/amd64 release. Milestones A through D retain their release evidence.

E1 and E2 of v0.2 are complete locally. The private queue store now has a FIFO coordinator, durable add/pause/resume/cancel/forget controls, live concurrency changes, conservative recovery, safe polling, and draining shutdown. Whole-transfer concurrency defaults to `1` and accepts `1..8`, independently of per-transfer connections. E3 shared bandwidth is next. Daemon IPC, CLI commands, and automatic startup are not implemented.

## Handoff

- `internal/queue/coordinator.go` implements `NewCoordinator(store, runner, parallel)`, `Run`, `List`, `Stop`, and `Done`. Construction durably reconciles active records and applies an optional startup override without dispatch; zero preserves the setting and a nil runner selects `transfer.Download`. Run executes once after startup prerequisites are ready. The caller owns the store and gives the coordinator exclusive mutation access until Run returns, then closes it.
- `controls.go` adds `Add`, `Pause`, `Resume`, `Cancel`, `Forget`, and `SetParallel`. Add takes a client ID and normalized transfer options with an absolute output. Retained IDs deduplicate by immutable request hash, including after terminal URL removal. Output reservations survive completion/cancellation; forget releases metadata/capacity without deleting files. Count, encoding, identity, and order limits reject before mutation. Resume consumes a fresh FIFO order only on a real transition.
- `lifecycle.go` owns recovery and outcome rules. Running recovers as interrupted, pausing as paused, and canceling as canceled, all with unknown recovered publication. Interrupted attempts need explicit resume. Persist running intent before engine calls and pause/cancel intent before normal cancellation. Verified publication wins late controls; published-with-error remains failed. An existing final file never proves completion or permits overwrite.
- `SetParallel` persists before changing admission. Increases admit FIFO work; decreases keep active transfers and withhold new admission until below the limit. Running, pausing, and canceling attempts keep their slots until the runner returns after cleanup. No queued-job goroutines, preemption, or automatic whole-job retries are added.
- One coordinator goroutine owns mutations, a 16-entry control mailbox, and at most eight runners. Progress stores only each attempt's latest value without waiting on disk or readers. `views.go` exposes owned safe records, with unknown progress after restart. Fixed error messages replace arbitrary runner/stored diagnostic text; requests, source URLs, and hashes never enter views.
- Caller cancellation after enqueue does not undo a mutation: unacknowledged mutations report `ErrOutcomeUnknown`. Buffered replies let abandoned callers leave without stalling the coordinator. While stopping, reads and repeated stop remain available; new mutations are refused. Stop acknowledges durable intent; Run/Done wait for all runners and result commits.
- Any persistence failure stops admission, cancels/joins runners, disables authoritative views, and prevents further saves. If pause intent cannot be persisted, the prior running record remains for conservative restart. `ErrPersistence` has fixed display text and wraps its cause for internal classification, including `ErrUncertain`/`ErrChanged`. The coordinator never closes the caller's store or writes older memory over an uncertain commit.
- E1 remains in `model.go`, `codec.go`, `store.go`, and `files_linux.go`: strict bounded private snapshots, immutable requests, monotonic order, owned copies, persistent nonblocking locks, and write/sync/rename/directory-sync commits. Unsupported queue-storage platforms return `ErrUnsupported`. `transfer.Normalize` remains shared pure validation. Full source URLs/query tokens persist privately by the owner's choice; terminal records clear URLs, while transfer manifests remain fingerprint-only.
- [Daemon and queue](docs/daemon-and-queue.md) owns the accepted contract. [Shared budgeting](docs/concurrency.md#daemon-wide-rate-cap) requires one daemon-lifetime body budget composed with each job's own cap; charge applicable budgets before waiting and never refund/reset credit on lifecycle or concurrency changes. The existing Runner seam can connect that work without importing queue/CLI code into the engine.
- Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. [Release evidence](docs/releases.md#stable-publication) owns prior qualification. Keep focused local commits; never push.

## Validation

- E2 lifecycle and coordinator slices each passed focused tests and `make check` before their local commits. The final `go test ./internal/queue -count=1` passed all model/store/lifecycle/coordinator/control and HTTP tests.
- The final `make check` passed formatting, vet, ordinary/race tests, and build. Queue and packaging suites reran; unchanged transfer, CLI, process, range, and transfer-storage suites reused matching cached results. No transfer-engine behavior was changed in E2.
- Deterministic runners cover FIFO ordering, all control/state combinations, restart transitions, settings through eight transfers, non-preemptive reductions, retained cleanup slots, idempotent add, output reservations, capacity/overflow/encoded bounds, and publication races. Injected failures cover preparation, admission, results, stop, uncertain commits, no stale saves, and joined cleanup. Blocked commits verify acknowledgment/admission ordering, bounded backlogs, nonblocking progress, and disconnected add/stop behavior.
- `TestQueueRealTransferPauseResumeAndNoOverwrite` uses the default engine runner and a local HTTP fixture: pause preserves an 8192-byte checkpoint, explicit resume sends the conditional suffix request, exact final bytes are published, terminal source tokens disappear, and a subsequent attempt preserves/refuses the existing output without another HTTP request.
- `CGO_ENABLED=0 go test ./internal/queue -count=1` passed outside the sandbox. Its initial sandboxed run failed because the HTTP fixture could not open a localhost listener (`operation not permitted`); the authorized rerun passed without code changes.
- The CI six-target compilation recipe passed with CGO disabled for Linux/macOS/Windows on amd64/arm64: `go build -p 2 -trimpath` for the CLI and `go test -p 2 -c` for every test package. Outputs are under `/tmp/yoinkdl-e2-compile.afL9c0`. Foreign binaries were not executed; native queue evidence remains Linux/amd64.
- `python3 /tmp/yoinkdl-doccheck.py` passed: 25 Markdown files, 327 local links/anchors, 32 shell blocks, six JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. The original design is unchanged, including its intentional Markdown hard breaks and illustrative placeholders.

## Next steps

1. Implement E3 shared transfer body budgets using the existing limiter/body boundaries. Preserve standalone behavior and compose each job's private cap with one shared daemon-lifetime budget. Add controlled-clock and local HTTP tests across independent transfers, probes, redirects, retries, cancellation, and job/concurrency changes; prove no refunds or renewed shared bursts.
2. Run focused E3 checks and `make check`, refresh this handoff, and split coherent local commits. Shared bandwidth remains an explicit startup setting, not a persistent queue setting or cross-process service.
3. Follow E4 Linux paths/private IPC/lifecycle, E5 CLI adapters, then E6 daemon process/platform qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order). Keep direct standalone commands independent and do not advertise daemon commands before the full path exists.

## Open decisions and blockers

R03/R04, Linux paths, and the owner's private URL/concurrency preferences are settled. No E2 blocker remains. E3 through E6 are unimplemented. Daemon IPC/CLI privacy, full daemon crash/publication-boundary qualification, native platforms beyond Linux/amd64, and real power-loss durability remain unverified. Signing and package-manager distribution remain separate work.
