# Roadmap

[Documentation index](README.md)

Milestones A through D are complete for the initial Linux/amd64 scope. Stable [0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) is published with verified public downloads and installation. v0.2 remains planned; there is no promised release date for later milestones.

## Delivery sequence

| Order | Milestone | Result |
|---|---|---|
| A | Reliable single stream | A useful foreground download command |
| B | Persistence and resume | Recovery from compatible local checkpoints |
| C | Multi-range engine | Safe bounded parallel transfers |
| D | v0.1 hardening | Validated UX, safety, platform behavior, and distribution |

Complete these in dependency order. A daemon or TUI should not delay the standalone engine.

## Milestone A — Reliable single stream

**Status:** implemented and tested on Linux/amd64. See [testing](testing.md) for the evidence and [CLI](cli.md) for working commands.

Deliver `download` and `version`, HTTP(S) streaming into a partial file, progress, cancellation, and verified final publication.

Reject bad HTTP responses, incomplete bodies, and destination conflicts. Use local-server integration tests with deterministic content.

Completion evidence: a direct download produces exactly the expected file; failed or interrupted transfers do not appear as completed output; publication obeys the chosen filesystem and overwrite policy.

The initial choices are Go 1.27, local module `yoinkdl`, standard-library parsing, exclusive partial creation, and hard-link/rename publication with syncs. Other platforms and broader crash-durability qualification remain release work.

## Milestone B — Persistence and resume

**Status:** implemented and tested on Linux/amd64, including actual forced-process termination and validated suffix recovery. The current CLI supports automatic compatible resume, explicit `--resume`, and read-only `inspect`.

Define the versioned manifest, interval convention, transfer identity, checkpoints, and exclusive ownership. Add server metadata probing, validator logic, safe sequential resumption when supported, and `inspect`.

Specify refusal or deliberate restart when state or server behavior is incompatible. Test normal interruption, process termination, malformed state, and recovery ordering.

Completion evidence: local-server tests resume terminated transfers to exact fixture hashes, reject changed identity/bounds without altering saved artifacts, and recover ready publication checkpoints. Kernel locks prevent competing writers and release after process death. Native macOS, Windows, network-filesystem, and power-loss qualification remain release work.

## Milestone C — Multi-range engine

**Status:** implemented and tested on Linux/amd64. Fresh/resumed transfers have strict identity validation, bounded workers and retries, exclusive offset writes, conservative checkpoints, cancellation, and verified publication. `--connections` accepts 1..32 (default four); `--retries` accepts 0..10 additional attempts (default three). Sequential and parallel resume have forced-process-termination evidence.

Plan disjoint chunks, bound worker concurrency, and write only assigned offsets. Validate every response's range, length, encoding, and compatible representation identity.

Add interval retries, bounded backoff, and safe sequential fallback. Exercise failures during concurrent transfer and run the race detector on meaningful paths.

Completion evidence: parallel, resumed, and retried outputs match exact fixtures. Tests verify shared attempt budgets, Retry-After/backoff bounds, whole-chunk replacement, unchanged completed coverage, terminal identity/storage failures, progress rollback, and cancellation while workers wait. The full gate, race checks, CLI process checks, and documentation audit pass. See [testing](testing.md).

## Milestone D — v0.1 hardening

**Status:** complete for the initial Linux/amd64 release. Stable `0.1.0` is published as Latest. Rate caps, JSON/text output, security regression fixes, native platform evidence, six-target compilation, all seven acceptance criteria, and real HTTPS subprocess tests are recorded. Hosted Ubuntu 24.04.5 CI passed with Go 1.27.0; [public stable downloads and installation](releases.md#stable-publication) passed. Native qualification beyond Linux/amd64 remains separate work.

Qualify native behavior and CI for any additional platform before claiming support, preserving the reviewed security contracts and documented trust boundaries. Cross-build success alone is insufficient; Windows storage remains unsupported.

The [stable review](releases.md#stable-readiness-review) found no reported regressions and checked actual acceptance evidence. The [0.1.0 release](releases.md#stable-publication) uses revision `31330c6b07d33af148341bdfb346c0e139082fdb`, preserving the candidate's runtime behavior and adding HTTPS qualification. Public files match the verified hosted assets exactly. The [changelog](../CHANGELOG.md#010) preserves both version sections and an Unreleased section; stable readiness was not inferred from lack of reports alone.

Completion evidence: all release acceptance criteria have recorded results, documented platform limits, and no unresolved integrity failure.

## v0.1.0 acceptance

The published Linux/amd64 release has recorded evidence for these criteria; future changes must preserve them:

- A direct HTTP(S) download produces the expected file.
- A supporting server resumes a compatible transfer across process termination.
- Range transfers never silently combine different resource versions.
- Wrong lengths and incomplete responses are detected.
- Verified files are published atomically under the qualified platform policy.
- Simultaneous writers cannot corrupt data or metadata.
- Integration and race tests pass on the required environments.

The [testing matrix](testing.md#required-scenarios) expands these into concrete scenarios. Installation examples and platform support claims need real verification before publication.

## v0.2 — Daemon and persistent queue

**Status:** E1 through E3 are implemented locally: private atomic queue storage, recovery, FIFO scheduling, durable controls, live concurrency, safe views, draining shutdown, and composed shared/per-transfer bandwidth. E4 Linux paths/private IPC/lifecycle is next, followed by CLI wiring. [Daemon and persistent queue](daemon-and-queue.md) owns R04: foreground Linux daemon, private local IPC, durable queue/source records, conservative recovery, and configurable whole-transfer concurrency. [Daemon-wide rate caps](concurrency.md#daemon-wide-rate-cap) owns R03. Neither changes the released standalone CLI or its transfer manifests.

Queue concurrency defaults to one transfer and persists across restarts. `daemon set --parallel N` selects `1..8` live; reductions wait for active work to finish before admitting more. The owner's full-URL persistence choice uses private queue files and redacted public views. Foreground startup is explicit, and no network service, database, TOML loader, TUI, or automatic daemon launch is required.

Pause/cancel use context cancellation and wait for the engine to release writers. Crash-interrupted attempts do not restart automatically or infer success from file presence. An initial queued transfer never overwrites a completed destination. `forget` explicitly frees a non-running queue record without deleting its data files.

### v0.2 implementation order

| Order | Complete slice | Required proof before moving on |
|---|---|---|
| E1 | Shared pure request validation; `internal/queue` job model, strict snapshot codec, and private Linux atomic store/lock | Same standalone validation behavior; exact schema/URL privacy rules; refused unsafe/corrupt state; durable mutations; uncertain-commit fail-stop; competing writers excluded |
| E2 | Queue coordinator, FIFO dispatcher, persisted concurrency setting, pause/resume/cancel/forget, and restart reconciliation | Fake-runner deterministic transitions; `1` serializes jobs; higher settings bound whole transfers; live increases/decreases and slot retention; persistence before side effects; publication/cancellation races |
| E3 | Optional shared transfer body budget with per-transfer composition | Combined cap across independent transfers/probes/retries; cancellation without refunds; unchanged standalone rate/timeout behavior; no credit reset when job/concurrency state changes |
| E4 | Linux state/runtime resolution, private Unix IPC, peer checks, both ownership locks, and safe startup/shutdown | Bounded strict protocol; state-directory mismatch refusal; stale/live/foreign socket preservation; peer/permission refusal; blocked/disconnected clients cannot stall or cancel jobs |
| E5 | CLI daemon/queue adapters and safe text/JSON views | Complete add/list/control path through real transfers; caller-relative output resolution; explicit startup; no source leakage; ambiguous acknowledgments; existing direct commands remain independent |
| E6 | Process-crash qualification, release docs, and platform gates | Real kill/restart and publication-boundary cases; queue-full/forget recovery; concurrency and shared-rate process tests; native Linux full/race/CGO-disabled gates and six-target compilation |

Keep each slice runnable and locally committed with its tests and current STATUS handoff. Use a narrow callable transfer runner for coordinator tests and retain one `transfer.Budget.Download` runner for the daemon lifetime. Do not add a generic job framework or duplicate the downloader. E4 is the immediate next implementation task; user-visible daemon commands should be documented as implemented only after their complete path exists.

The [v0.2 validation matrix](testing.md#planned-v02-validation) makes the failure paths concrete. A daemon release requires all slices and evidence, not merely a listener or a queue file.

## v0.3 — TUI and richer local workflows

Add a TUI, named profiles, richer inspection/history, and optional completion hooks. The TUI consumes snapshots/events rather than worker internals.

Hooks need a separate opt-in execution policy. Arbitrary workflow languages remain outside this release's required scope.

## Later front ends

These are proposed future commands, not available v0.1 commands:

```text
yoinkdl daemon start
yoinkdl daemon stop
yoinkdl add <url>
yoinkdl list
yoinkdl pause <id>
yoinkdl resume <id>
yoinkdl cancel <id>
yoinkdl tui
```

`daemon start` is the primary name and will run in the foreground. The accepted [v0.2 control contract](daemon-and-queue.md#local-control-protocol) uses a private Linux Unix socket and versioned JSON frames; no TCP port or public HTTP API exists. `daemon set --parallel` and `forget` are also planned controls. The historical `summon` alias remains deferred.

## Deferred concepts

YoinkDL carries forward the reliability goals of the earlier GoonDL design while keeping its initial implementation small.

| Earlier or proposed concept | Current disposition |
|---|---|
| Adaptive chunks and round-robin scheduling | Start with static ranges; revisit with measurements |
| `.goondl-meta` and a custom extension ecosystem | Begin with versioned `.yoinkdl.json`; avoid custom languages |
| `~/.config/goondl` | Proposed `~/.config/yoinkdl`, with transfer sidecars beside destinations |
| `goonprofile`, `goonhook`, related profiles/hooks | v0.3 or later, starting with simple declarative formats |
| JavaScript, Lua, or WASM workflows | Unscheduled; requires runtime and security design |
| HTMX/web monitoring dashboard | Unscheduled optional daemon front end |
| SFTP, SCP/SSH, torrents, Metalink | Future protocol/transport work; no delivery commitment |
| Remote nodes, LAN swarm, P2P, peer discovery | Separate future transfer architecture |
| Scheduling and sandboxed automation | Unscheduled beyond the initial queue scope |
| Browser automation and protected-content handling | Out of scope |
| Paranoia Mode, voice notifications, mobile UI | Not on the delivery roadmap |
| Arbitrary-precision size-expression parser | Separate optional utility; not a v0.1 blocker |
| User checksum flag, cleanup command, short URL alias | Deferred until their interfaces are justified and specified |

Track unresolved implementation contracts in [open decisions](decisions.md), rather than treating a deferred feature as an implicit dependency of v0.1.
