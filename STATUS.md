# Project status

Updated: October 14, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E4 are complete locally.

The internal Linux daemon now owns private state/runtime paths, validates bounded local IPC, dispatches queue controls, composes shared bandwidth, and drains transfers and clients before releasing ownership. E5 command adapters, safe text/JSON results, and local integration coverage are implemented. Final process-qualification review and its separate local commit remain. Automatic startup is not implemented; E6 full daemon process/crash qualification remains pending.

## Handoff

- `internal/cli/daemon.go`, `queue.go`, and `queue_output.go` implement explicit foreground start/stop/set and add/list/pause/resume/cancel/forget through the existing daemon API. Shared transfer parsing feeds normalized immutable requests; add generates random 128-bit IDs and resolves outputs in the client's directory.
- All queue client commands support one version-1 JSON result carrying command, job ID, exit code, safe result, and fixed error. Text output escapes nonprinting paths; add prints only the ID. Lost/malformed/mismatched acknowledgments preserve the submitted ID and unknown outcome. Failed client output never sends job cancellation. Startup reports paths/fallback and admits no jobs if its readiness output fails.

- `internal/cli/flags.go` extracts the existing interspersed flag scanner. It applies only registered options, preserves help/terminator behavior, and reports invalid recognized flag names without reflecting values. `parseDownload` uses it with unchanged defaults and validation; new queue/daemon parsers will reuse it.

- `internal/daemon/paths.go`, `paths_linux.go`, and `paths_other.go` expose pure `Resolve()` with normalized state/runtime paths and `Fallback`. Relative/invalid XDG values are ignored; unsafe configured runtime bases are refused. Overlong socket paths fail before creation. Non-Linux resolution, construction, and client calls return `ErrUnsupported`.
- `ownership_linux.go` reuses `queue.Open`, creates private runtime directories, and holds a separate persistent nonblocking lock. Socket probing/binding uses `/proc/self/fd` anchors; initial chmod uses an O_PATH socket handle. Linux `/proc/self/fd` is required. Stale removal requires both locks, socket type/owner/identity checks, and connection-refused. Cleanup preserves foreign or replaced artifacts. Peer checks use kernel effective-user credentials in both directions.
- `protocol.go`, `json.go`, and `replies.go` enforce version-1 newline framing: 64 KiB requests, 8 MiB replies, exact nested fields, non-null required values, valid Unicode, normalized transfer settings, state-root matching, and operation-specific safe replies. Raw parser/runner/source messages are never reflected. Only the first frame is dispatched; pipelined mutations are not executed.
- `daemon.New(Options{Paths, Parallel, Limit})` validates settings, acquires both locks, durably reconciles recovery/startup concurrency, and binds the socket without dispatch. `Server.Run(ctx)` executes once and starts the coordinator only after preparation. Keep one retained `Budget.Download` runner for the runtime lifetime. Startup failure releases ownership and preserves any already-committed recovery/settings.
- `server_linux.go` bounds accepted clients to sixteen, uses five-second socket read/write deadlines, and separates client waits from job contexts. It exposes ping/list/add/pause/resume/cancel/forget/set_parallel/stop through the existing coordinator. Stop acknowledges durable intent; reads/repeated stop remain available while transfers drain. Persistence or detected runtime ownership failure stops and joins work without stale saves.
- `Close` releases an unstarted server or requests joined shutdown through an independent signal, so control-mailbox pressure cannot block the request. `Done` closes after engine results, accepted client handlers, socket cleanup, and both lock releases. Parent-context cancellation remains an interruption error; clean stop/Close returns nil. CLI signal/exit-code mapping now uses that result after cleanup.
- `Call(ctx, paths, op, args)` validates locally, checks private endpoint/peer identity, sends once, and validates the reply. Empty operations require an empty args object. Calls create no directories/processes and do not retry. Failed writes or lost/malformed replies after a mutation attempt wrap `queue.ErrOutcomeUnknown`; fixed remote failures expose `Failure.Code` and `Message`. Client cancellation/disconnection never undoes accepted jobs.
- E1/E2 queue contracts remain unchanged: durable intents before effects, whole-transfer concurrency `1..8`, retained cleanup slots, explicit resume after interruption, safe views, private full source URLs, terminal URL removal, and metadata-only forget. E3 retains private/shared body caps across attempts and queue controls. Standalone commands and transfer manifests remain independent.
- [Daemon and queue](docs/daemon-and-queue.md) owns the accepted runtime/protocol contract; [CLI contract](docs/cli.md#daemon-and-queue) documents the implemented adapters. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- `go test ./cmd/yoinkdl ./internal/cli -run 'Test(Daemon|Queue)' -count=1` passed. Tests cover command validation/redaction, caller-relative/default outputs, real HTTP pause/resume/cancel, exact publication, no overwrite, no implicit startup, safe text/JSON, lost and mismatched acknowledgments, and failed/blocked output without job cancellation.
- `make check` passed formatting, vet, ordinary/race tests, and build, including the process fixtures currently pending their separate commit. Changed CLI/process and packaging suites ran; unchanged daemon/queue/transfer/storage/range suites reused matching cached results.
- `CGO_ENABLED=0 go test ./cmd/yoinkdl ./internal/cli -count=1` passed. The CI six-target CGO-disabled CLI/test compilation recipe passed under `/tmp/yoinkdl-e5-compile.zrqbt3eo`; foreign binaries were not executed.
- `python3 /tmp/yoinkdl-doccheck.py` passed 25 Markdown files, 338 local links/anchors, 34 shell blocks, eight JSON and two TOML examples, fences, and whitespace. A draft trailing blank line was removed; `git diff --check` passed. E1 through E4 retain their evidence in [testing](docs/testing.md).

## Next steps

1. Finish documentation/review and commit the already-passing `cmd/yoinkdl/daemon_linux_test.go` process qualification separately. It covers actual commands, clean restart, SIGINT/SIGTERM, broken/full output pipes, preserved work, and startup output cleanup.
2. Refresh the final E5 handoff and current documentation. Do not repeat implementation gates unless a code change or unresolved concern requires it.
3. Continue E6 forced-death/publication-boundary, process concurrency/shared-rate, and final platform/release qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order).

## Open decisions and blockers

No implementation blocker remains. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E5 final handoff/qualification commit and E6 remain unfinished. Full daemon crash/publication-boundary qualification, native platforms beyond Linux/amd64, and real power-loss durability remain unverified. Signing and package-manager distribution remain separate work.
