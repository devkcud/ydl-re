# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1/E2 are complete locally.

E3 is in progress. The engine now composes one reusable shared body budget with each transfer's private cap. Controlled-clock tests and concurrent-download HTTP coverage are implemented; extended protocol/lifecycle qualification remains before completing E3. Daemon paths, IPC, CLI commands, and automatic startup are unimplemented.

## Handoff

- `internal/transfer/limit.go` exposes `NewBudget(rate)` and a concurrency-safe `Budget`. Zero returns a nil unlimited handle; negative rates fail. Keep the same pointer for the owner's lifetime and do not copy initialized budgets. Nil/zero-value handles add no shared cap.
- `Budget.Download` matches `queue.Runner`; use `queue.NewCoordinator(store, budget.Download, parallel)` at the future daemon boundary. Each call creates a fresh private request cap while preserving shared credit/debt. Standalone `transfer.Download` delegates with a nil shared handle. Requests, hashes, snapshots, and manifests are unchanged.
- `body.go` bounds reads by the smallest applicable burst, reserves actual bytes against every budget before waiting or returning cancellation, and waits until the latest eligibility time. Reads returning bytes plus an error still charge both budgets. No refunds, rate mutation, or extra worker goroutines are introduced. Throttle waits remain outside body inactivity timers.
- E1/E2 remain in `internal/queue`: private bounded atomic snapshots and persistent Linux locks; FIFO coordinator with durable controls and live whole-transfer concurrency `1..8`; conservative recovery, safe polling, bounded progress/control mailboxes, and draining shutdown. The caller owns the store until Run returns. Nil runner selects standalone `transfer.Download`.
- Queue publication and persistence rules remain authoritative: persist intents before effects, retain slots until engine cleanup, preserve ambiguous recovered publication, require explicit interrupted-job resume, and never overwrite final files. Persistence failure stops admission, joins runners, disables authoritative views, and prevents stale saves. Retained output reservations clear only through forget; terminal records clear private source URLs.
- [Daemon and queue](docs/daemon-and-queue.md) and [shared budgeting](docs/concurrency.md#daemon-wide-rate-cap) own the accepted contracts. Shared rate is an explicit startup setting, separate from persistent queue concurrency. Linux IPC/path selection, daemon views, and command adapters belong to E4/E5.
- Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. [Release evidence](docs/releases.md#stable-publication) owns prior qualification. Keep focused local commits; never push.

## Validation

- `go test ./internal/transfer -run 'Test(Limiter|ParseLimit|Body|SharedBudget|BudgetComposition)' -count=1` passed for the initial composition slice before the HTTP tests were added. Controlled time covers four independent clients with four readers each, private/shared/both caps, minimum/maximum rates, error/cancellation charges, latest-deadline waits, timeout separation, and retained idle credit.
- `go test ./internal/transfer -run 'Test(SharedBudgetDownload|ZeroBudgetDownload)' -count=1` passed outside the sandbox. The initial sandboxed run could not open a localhost listener (`operation not permitted`); no code change was needed for the rerun.
- `make check` passed formatting, vet, ordinary/race tests, and build. The changed transfer engine, queue integration, CLI/process, and packaging suites ran; unchanged ranges/storage suites reused matching cached results.
- `python3 /tmp/yoinkdl-doccheck.py` checks all 25 Markdown files for local links/anchors, fences, shell/JSON/TOML example syntax, and whitespace. Illustrative shell placeholders are substituted for syntax checking; the original design's intentional hard breaks are retained. `git diff --check` passed.
- Prior E1/E2 ordinary/race/CGO-disabled checks and six-target CLI/test compilation passed. Those are prior-slice evidence, not qualification of the new shared-budget work. Foreign binaries were not executed.

## Next steps

1. Finish E3 protocol/lifecycle coverage: independent transfers across probes, redirects, retries, fallback, saved/offline recovery, cancellation, and queue pause/resume/failure/completion/concurrency changes. Prove no refunds or renewed shared bursts.
2. Run focused E3 checks and `make check`, refresh this handoff, and split coherent local commits. Validate local documentation links/anchors, examples, and whitespace.
3. Follow E4 Linux paths/private IPC/lifecycle, E5 CLI adapters, then E6 daemon process/platform qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order). Keep direct standalone commands independent and do not advertise daemon commands before the full path exists.

## Open decisions and blockers

R03/R04, Linux paths, and private URL/concurrency preferences are settled. No design blocker remains. E3 qualification and E4 through E6 remain unfinished. IPC/CLI privacy, full daemon crash/publication-boundary qualification, native platforms beyond Linux/amd64, and real power-loss durability remain unverified. Signing and package-manager distribution remain separate work.
