# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E3 are complete locally.

The engine composes one reusable shared body budget with each transfer's private cap. Controlled-clock and local HTTP tests cover independent transfers, protocol traffic, recovery, and queue lifecycle/concurrency changes. E4 is in progress: Linux path resolution, private runtime ownership, conservative socket handling, and peer checks are implemented. IPC framing/dispatch and daemon lifecycle wiring are next. Daemon CLI commands and automatic startup remain unimplemented.

## Handoff

- `internal/daemon/paths.go`, `paths_linux.go`, and `paths_other.go` add pure `Resolve` with normalized state/runtime paths and a fallback flag. Relative/invalid XDG values are ignored; unsafe configured runtime bases fail at ownership acquisition. Overlong socket paths fail before artifact creation. Non-Linux resolution returns `ErrUnsupported`.
- `ownership_linux.go` reuses `queue.Open`, creates private runtime directories, holds a separate nonblocking persistent lock, and anchors endpoint IO through `/proc/self/fd`. Stale removal requires both locks, socket owner/type/identity checks, and connection-refused. Live/unsafe/replaced paths are preserved; cleanup removes only the created socket inode. Permission changes use an O_PATH socket handle. Kernel peer-UID checks are ready for both client and server integration.

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

- `go test ./internal/daemon -count=1` passed outside the sandbox. The initial run was blocked by Unix-socket `setsockopt: operation not permitted`; the authorized rerun passed without code changes.
- Ownership tests cover path normalization/fallback, overlong paths, private directories/locks, both ownership-lock combinations, stale/live/unsafe/replaced sockets, changed roots/locks, simulated foreign owners, and kernel peer-UID matches/refusals.
- `make check` passed formatting, vet, ordinary/race tests, and build for this ownership slice. `python3 /tmp/yoinkdl-doccheck.py` passed 25 Markdown files, 333 local links/anchors, 32 shell blocks, six JSON and two TOML examples, fences, and whitespace; `git diff --check` passed. E1 through E3 retain their recorded ordinary/race/CGO-disabled evidence in [testing](docs/testing.md); final E4 platform/process qualification is still pending.

## Next steps

1. Implement strict bounded IPC request/reply framing and fixed safe responses using the accepted protocol. Reject unsupported versions, duplicate/unknown/null fields, malformed UTF-8/JSON, wrong operation shapes, and state-root mismatches without reflecting private source data.
2. Wire request dispatch and lifecycle after both locks and the endpoint are ready. Retain one shared budget/coordinator, isolate client deadlines, and stop admission/join transfers/persist outcomes before endpoint cleanup. Add the matching peer-checked client boundary for E5.
3. Validate E4 failure/ownership/IPC behavior and `make check`, refresh this handoff, and split coherent local commits. Follow E5 CLI adapters and E6 daemon process/platform qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order).

## Open decisions and blockers

R03/R04, Linux paths, and private URL/concurrency preferences are settled. No design blocker remains. E4 framing/lifecycle and E5/E6 are unfinished. IPC/CLI privacy, full daemon crash/publication-boundary qualification, native platforms beyond Linux/amd64, and real power-loss durability remain unverified. Signing and package-manager distribution remain separate work.
