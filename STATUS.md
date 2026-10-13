# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E3 are complete locally.

The engine composes one reusable shared body budget with each transfer's private cap. Controlled-clock and local HTTP tests cover independent transfers, protocol traffic, recovery, and queue lifecycle/concurrency changes. E4 Linux paths/private IPC/lifecycle is next. Daemon IPC, CLI commands, and automatic startup remain unimplemented.

## Handoff

- `internal/transfer/limit.go` exposes `NewBudget(rate)` and a concurrency-safe `Budget`. Zero returns a nil unlimited handle; negative rates fail. Keep the same pointer for the owner's lifetime and do not copy initialized budgets. Nil/zero-value handles add no shared cap.
- `Budget.Download` matches `queue.Runner`; use `queue.NewCoordinator(store, budget.Download, parallel)` at the future daemon boundary. Each call creates a fresh private request cap while preserving shared credit/debt. Standalone `transfer.Download` delegates with a nil shared handle. Requests, hashes, snapshots, and manifests are unchanged.
- `body.go` bounds reads by the smallest applicable burst, reserves actual bytes against every budget before waiting or returning cancellation, and waits until the latest eligibility time. Reads returning bytes plus an error still charge both budgets. No refunds, rate mutation, or extra worker goroutines are introduced. Throttle waits remain outside body inactivity timers.
- `budget_test.go` and `budget_http_test.go` cover independent clients/downloads and composed allowances. `body_test.go` and `limit_integration_test.go` now run private/shared accounting through redirects, failed probes/chunks, fallback, saved bytes, retries, cancellation, and HTTP resume. `budget_recovery_test.go` proves ready recovery is offline and neither charges nor resets credit.
- `internal/queue/budget_http_linux_test.go` retains one budget through real pause/resume, failure/retry, cancellation, completion, subsequent jobs, and concurrency increases/reductions. It checks the cumulative allowance and exact final/partial bytes. Queue production code needed no change: the existing runner boundary owns the shared handle.
- E1/E2 remain in `internal/queue`: private bounded atomic snapshots and persistent Linux locks; FIFO coordinator with durable controls and live whole-transfer concurrency `1..8`; conservative recovery, safe polling, bounded progress/control mailboxes, and draining shutdown. Construction prepares recovery without dispatch; call Run once after startup prerequisites are ready. The caller owns the store until Run returns. Nil runner selects standalone `transfer.Download`.
- Queue publication and persistence rules remain authoritative: persist intents before effects, retain slots until engine cleanup, preserve ambiguous recovered publication, require explicit interrupted-job resume, and never overwrite final files. Persistence failure stops admission, joins runners, disables authoritative views, and prevents stale saves. Retained output reservations clear only through forget; terminal records clear private source URLs.
- [Daemon and queue](docs/daemon-and-queue.md) and [shared budgeting](docs/concurrency.md#daemon-wide-rate-cap) own the accepted contracts. Shared rate is an explicit startup setting, separate from persistent queue concurrency. Linux IPC/path selection, daemon views, and command adapters belong to E4/E5.
- Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. [Release evidence](docs/releases.md#stable-publication) owns prior qualification. Keep focused local commits; never push.

## Validation

- The implementation and qualification slices each passed `make check`: formatting, vet, ordinary/race tests, and build. The initial gate reran transfer, queue, CLI/process, and packaging suites; the final gate reran transfer/queue/packaging tests and reused matching cached results for unchanged CLI/process/ranges/storage packages.
- Controlled-clock checks passed: `go test ./internal/transfer -run 'Test(Limiter|ParseLimit|Body|SharedBudget|BudgetComposition)' -count=1` for the initial composition; `go test ./internal/transfer -run 'Test(RateBudget|RateWait|BodyBudget)' -count=1` for extended protocol accounting; `go test ./internal/transfer -run TestSharedBudgetReadyRecoveryIsOfflineAndFree -count=1` for both ready formats.
- Local HTTP checks passed: `go test ./internal/transfer -run 'Test(SharedBudgetDownload|ZeroBudgetDownload)' -count=1`, `go test ./internal/transfer -run 'TestDownloadRateLimitPaths' -count=1`, and `go test ./internal/queue -run TestQueueSharedBudgetSurvivesLifecycleAndConcurrency -count=1`. The first HTTP run could not open a sandboxed localhost listener (`operation not permitted`); its authorized rerun passed without code changes.
- `CGO_ENABLED=0 go test ./internal/transfer ./internal/queue -count=1` passed outside the sandbox. Native E3 runtime evidence is Linux/amd64. Six-target CLI/test compilation remains prior E1/E2 evidence; E3 changes no platform-specific implementation, and E6 retains final platform/process qualification. Foreign binaries were not executed.
- `python3 /tmp/yoinkdl-doccheck.py` passed: 25 Markdown files, 332 local links/anchors, 32 shell blocks, six JSON examples, two TOML examples, fences, and whitespace. Illustrative shell placeholders were substituted for syntax checking; the original design's intentional hard breaks are retained. `git diff --check` passed.

## Next steps

1. Implement E4 Linux state/runtime resolution and ownership using the [accepted paths](docs/daemon-and-queue.md#paths-and-access): absolute XDG selection/fallback, private owned directories, separate state/runtime locks, and conservative stale/live/foreign socket handling. Keep startup failures non-destructive.
2. Add bounded private Unix IPC with kernel peer-UID and state-root checks, strict versioned frames, fixed safe errors, and independent client deadlines. Wire one retained budget/coordinator only after startup prerequisites succeed; shutdown must stop admission, join transfers, persist outcomes, and release endpoint ownership safely.
3. Run focused E4 failure/ownership/IPC checks and `make check`, update this handoff, and split coherent local commits. Then follow E5 CLI adapters and E6 daemon process/platform qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order). Keep direct standalone commands independent and do not advertise daemon commands before their full path exists.

## Open decisions and blockers

R03/R04, Linux paths, and private URL/concurrency preferences are settled. No E3 blocker remains. E4 through E6 are unimplemented. IPC/CLI privacy, full daemon crash/publication-boundary qualification, native platforms beyond Linux/amd64, and real power-loss durability remain unverified. Signing and package-manager distribution remain separate work.
