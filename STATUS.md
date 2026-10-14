# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E4 are complete locally.

The internal Linux daemon now owns private state/runtime paths, validates bounded local IPC, dispatches queue controls, composes shared bandwidth, and drains transfers and clients before releasing ownership. E5 is in progress: common redacted option parsing is extracted; daemon/queue adapters are next. Daemon/queue commands and automatic startup remain unimplemented; E6 full daemon process/crash qualification remains pending.

## Handoff

- `internal/cli/flags.go` extracts the existing interspersed flag scanner. It applies only registered options, preserves help/terminator behavior, and reports invalid recognized flag names without reflecting values. `parseDownload` uses it with unchanged defaults and validation; new queue/daemon parsers will reuse it.

- `internal/daemon/paths.go`, `paths_linux.go`, and `paths_other.go` expose pure `Resolve()` with normalized state/runtime paths and `Fallback`. Relative/invalid XDG values are ignored; unsafe configured runtime bases are refused. Overlong socket paths fail before creation. Non-Linux resolution, construction, and client calls return `ErrUnsupported`.
- `ownership_linux.go` reuses `queue.Open`, creates private runtime directories, and holds a separate persistent nonblocking lock. Socket probing/binding uses `/proc/self/fd` anchors; initial chmod uses an O_PATH socket handle. Linux `/proc/self/fd` is required. Stale removal requires both locks, socket type/owner/identity checks, and connection-refused. Cleanup preserves foreign or replaced artifacts. Peer checks use kernel effective-user credentials in both directions.
- `protocol.go`, `json.go`, and `replies.go` enforce version-1 newline framing: 64 KiB requests, 8 MiB replies, exact nested fields, non-null required values, valid Unicode, normalized transfer settings, state-root matching, and operation-specific safe replies. Raw parser/runner/source messages are never reflected. Only the first frame is dispatched; pipelined mutations are not executed.
- `daemon.New(Options{Paths, Parallel, Limit})` validates settings, acquires both locks, durably reconciles recovery/startup concurrency, and binds the socket without dispatch. `Server.Run(ctx)` executes once and starts the coordinator only after preparation. Keep one retained `Budget.Download` runner for the runtime lifetime. Startup failure releases ownership and preserves any already-committed recovery/settings.
- `server_linux.go` bounds accepted clients to sixteen, uses five-second socket read/write deadlines, and separates client waits from job contexts. It exposes ping/list/add/pause/resume/cancel/forget/set_parallel/stop through the existing coordinator. Stop acknowledges durable intent; reads/repeated stop remain available while transfers drain. Persistence or detected runtime ownership failure stops and joins work without stale saves.
- `Close` releases an unstarted server or requests joined shutdown through an independent signal, so control-mailbox pressure cannot block the request. `Done` closes after engine results, accepted client handlers, socket cleanup, and both lock releases. Parent-context cancellation remains an interruption error; clean stop/Close returns nil. CLI signal/exit-code mapping is E5 work.
- `Call(ctx, paths, op, args)` validates locally, checks private endpoint/peer identity, sends once, and validates the reply. Empty operations require an empty args object. Calls create no directories/processes and do not retry. Failed writes or lost/malformed replies after a mutation attempt wrap `queue.ErrOutcomeUnknown`; fixed remote failures expose `Failure.Code` and `Message`. Client cancellation/disconnection never undoes accepted jobs.
- E1/E2 queue contracts remain unchanged: durable intents before effects, whole-transfer concurrency `1..8`, retained cleanup slots, explicit resume after interruption, safe views, private full source URLs, terminal URL removal, and metadata-only forget. E3 retains private/shared body caps across attempts and queue controls. Standalone commands and transfer manifests remain independent.
- [Daemon and queue](docs/daemon-and-queue.md) owns the accepted runtime/protocol contract; [CLI additions](docs/cli.md#planned-v02-controls) describe the next adapters. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- `go test ./internal/cli -run 'Test(Commands|DownloadOptions|InvalidArguments|ConnectionOptions|RetryOptions|RateOptions|InvalidOptions)' -count=1` passed outside the sandbox, covering existing parser behavior, redaction, option ordering, and HTTP transfer fixtures.
- `make check` passed formatting, vet, ordinary/race tests, and build for the parser extraction. Existing E1 through E4 evidence remains in [testing](docs/testing.md); E5 CLI behavior is not yet implemented.
- `python3 /tmp/yoinkdl-doccheck.py` passed 25 Markdown files, 337 local links/anchors, 32 shell blocks, six JSON and two TOML examples, fences, and whitespace. `git diff --check` passed.

## Next steps

1. Implement E5 explicit daemon start/stop/set and add/list/pause/resume/cancel/forget CLI adapters using `Resolve`, `New`/`Run`, and `Call`. Reuse current option/rate parsing and pure request normalization. Generate client IDs, resolve outputs against the client's working directory, report runtime fallback, and preserve no-overwrite/no-implicit-start behavior.
2. Add safe text/JSON views and correct interruption/error handling. Preserve unknown mutation outcomes and the submitted job ID when replies disappear; broken client output must not cancel durable work. Keep source URLs out of all output/errors and retain existing standalone command behavior.
3. Validate complete commands through real transfers, privacy, disconnected replies/output failures, and signal cleanup; run `make check`, refresh this handoff, and split local commits. Then execute E6 forced-death/publication-boundary, process concurrency/shared-rate, and final platform/release qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order).

## Open decisions and blockers

No implementation blocker remains. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E5/E6 remain unfinished. CLI privacy/rendering, full daemon process/crash/publication-boundary qualification, native platforms beyond Linux/amd64, and real power-loss durability remain unverified. Signing and package-manager distribution remain separate work.
