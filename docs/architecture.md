# Architecture

[Documentation index](README.md)

The current CLI/daemon and pure configuration/application foundations implement the packages below. The diagrams and proposed layout describe the larger target architecture.

## Current packages

| Package | Implemented responsibility |
|---|---|
| `cmd/yoinkdl` | Entry point, build version, SIGINT/SIGTERM cancellation |
| `internal/cli` | Standalone and daemon/queue parsing, inspection, safe text/JSON results, stdout/stderr, progress, and output backpressure |
| `internal/transfer` | Sequential/ranged HTTP, probing, immutable identity, response validation, retries, timeouts, composed transfer/shared rate budgets, byte checks, snapshots |
| `internal/ranges` | Bounded fixed-chunk planning, validated coverage, missing work, HTTP endpoint conversion |
| `internal/storage` | Stable OS locks, prefix/chunk manifests and checksums, durable checkpoints, preservation, publication recovery |
| `internal/queue` | Private atomic state, FIFO admission, durable controls, live concurrency, recovery, safe polling, and draining shutdown |
| `internal/daemon` | Linux path/endpoint ownership, strict private IPC, peer checks, bounded clients, safe views, and joined runtime lifecycle |
| `internal/config` | Pure typed settings, exact lookup, inherited resolution/origins, owned scoped edits, and bounded strict JSON codec; no tokens/IO/CLI integration |
| `internal/app` | Earlier pure version-1 metadata model/codec/update guards; the selected version-2 ledger and filesystem store remain future work |
| `internal/jsoncheck` | Shared lossless UTF-8/surrogate validation before schema-specific JSON decoding |

`transfer.Download` is the current application entry point. Storage verifies local bytes before transfer validates remote identity; only then may it truncate uncertain tails. `storage.Inspect` reads metadata without acquiring writer ownership or contacting the server. Existing transfers do not consume the new config model or an app store. Tests live beside the code, including interruption and forced-process-crash checks in `cmd/yoinkdl`.

`transfer.Normalize` shares the engine's pure URL/option validation, defaults, and automatic filename rules with queue callers. It removes fragments, owns the resolved retry value, preserves explicit output strings, and performs no file/network work. `Download` uses the same normalization internally; filesystem ownership and absolute output resolution still belong to their existing boundaries.

`cli/daemon.go` prepares the foreground owner through `daemon.New`, writes the escaped startup notice, then calls `Run`. Failed startup output closes ownership before admission. `cli/queue.go` reuses transfer/flag validation, generates client job IDs, resolves absolute outputs in the client's directory, and sends one `daemon.Call`. `queue_output.go` prints safe text or one versioned JSON result, keeping submitted IDs and uncertain outcomes independent of acknowledgment-output failures. Client output never owns a job's cancellation context.

`transfer.NewBudget` creates an optional shared body budget. Its `Download` method has the same signature as the standalone function and can serve directly as a queue runner. Each invocation creates its private request budget, while the receiver retains shared credit/debt across invocations. The HTTP body wrapper charges both before waiting until the later reservation; no queue types or shared-budget state enter transfer requests or manifests.

`queue.Open` owns a private state directory and persistent Linux file lock. `Store.Save` validates copied candidates and retained request identities, writes and syncs a temporary snapshot, renames it, and syncs the directory before replacing its committed cache. `Snapshot` returns a deep copy. Lost artifact ownership or uncertain commits disable the store until close/reopen. This layer never calls the engine, reconciles job states, or chooses XDG paths.

`queue.NewCoordinator` durably prepares recovery and startup settings; `Run` then owns FIFO admission, attempt contexts, result commits, and shutdown. It calls a narrow runner (the existing downloader by default), exposes safe polling views, and samples coalesced progress without blocking the engine on disk or readers. Any persistence failure stops admission and joins active runners without further saves. The caller retains store ownership until Run returns.

`controls.go` exposes add, pause/resume/cancel/forget, and live `SetParallel` through the bounded coordinator mailbox. Add deduplicates retained IDs by immutable request hash; forgotten records release capacity and output reservations without deleting files. Resume consumes a new FIFO order only on a real state transition. Control candidates must fit snapshot bounds before persistence, and cancellation follows a successful intent commit. Reducing concurrency retains active attempts and blocks new admission until there is room.

The chunk storage API provides exclusive `ClaimRange` writers with bounded offset writes and serialized `Commit`. Workers validate each completed HTTP response before committing; closing a writer releases an incomplete attempt without acknowledging it. `FinalizeContext` checks coverage and hashes with cancellation. File closure and publication require all writers to release their claims.

For fresh transfers requesting multiple connections, `Download` calls `probe` after acquiring output ownership. It starts version-2 storage and `receiveRanges` for a selected representation, or consumes/closes the returned full stream with its own metadata and sequential storage. `representation.request` and `representation.validate` bind each worker to that immutable identity.

The pool has at most 32 workers, 256 queued intervals, and one 64 KiB payload buffer per worker. Progress events use a channel bounded by worker count; only the coordinator invokes callbacks. Terminal failure cancels peers and the coordinator joins every worker before storage closes or publishes. Existing sequential resume stays on its original path.

`retry.go` classifies failures at network boundaries so local storage errors cannot enter the retry loop. Probe/validation operations and whole chunks each own one budget; full-stream setup has a separate request budget. Failed responses and writer claims close before backoff. A chunk keeps its worker slot, buffer, identity, and interval across attempts; only terminal or exhausted failures cancel peers. Progress rolls back failed bytes before replacement.

`limit.go` parses byte rates and owns one shared budget per `Download`. The HTTP transport wrapper in `body.go` gives every response its own cancellation context, bounds reads by the burst allowance, times only the underlying read, and charges actual body bytes before delivery. Probes, redirect drains, fallback streams, and retry bodies all use that budget. Progress rollback and saved bytes cannot supply rate credit. Closing a body cancels its request; the wrapper forwards idle-connection cleanup to the underlying transport.

For version-2 resume, storage verifies local chunks before `range_resume.go` validates a conditional response against the saved identity. Only then does `ContinueRanges` activate the same bounded pool over whole missing chunks. The coordinator emits initial saved progress and checks cancellation before starting workers. Saved-byte totals contribute to percentage, while probes and reused bytes are excluded from speed.

## Component flow

```mermaid
flowchart TD
    CLI[CLI and signal handling] --> App[Application service]
    App --> Core[Transfer coordinator]
    Core --> HTTP[HTTP probe and validation]
    Core --> Plan[Range planner]
    Core --> Store[State store]
    Core --> Pool[Scheduler and bounded workers]
    Plan --> Pool
    Pool --> HTTP
    Pool --> Writer[File writer]
    Writer --> Part[Partial file]
    Store --> Manifest[Manifest and checkpoints]
    Core --> Verify[Verification and finalization]
    Part --> Verify
    Verify --> Final[Final output]
    Core --> Events[Progress and result events]
    Events --> CLI
```

The CLI constructs a request and owns terminal interaction. The application service manages one transfer lifecycle. The coordinator chooses a plan, drives workers, persists safe progress, and authorizes finalization.

## Responsibilities

| Component | Owns |
|---|---|
| CLI | Parsing, display, error presentation, version command, process signals |
| Application service | Input validation, effective settings, lifecycle orchestration |
| HTTP transport | Probes, redirects, requests, response validation, timeout behavior |
| Planner | Sequential/ranged mode selection and non-overlapping intervals |
| Scheduler | Worker limits, task ownership, retry timing, cancellation |
| Storage | Destination policy, partial files, transfer identity, locks, checkpoints, final publication |
| Progress | Events and snapshots for CLI progress and safe daemon views |

These are responsibilities, not a requirement for a type or interface per row. Add a package only when its ownership is distinct.

## Proposed repository layout

```text
yoinkdl/
├── cmd/
│   └── yoinkdl/
│       └── main.go
├── internal/
│   ├── app/
│   ├── transfer/
│   ├── httpx/
│   ├── storage/
│   ├── progress/
│   └── config/
├── test/
│   ├── integration/
│   └── fixtures/
├── docs/
├── go.mod
├── README.md
└── LICENSE
```

| Proposed package | Scope |
|---|---|
| `cmd/yoinkdl` | Entry point and command wiring |
| `internal/app` | Use cases and lifecycle boundaries |
| `internal/transfer` | Coordinator, planner, scheduler, workers |
| `internal/httpx` | HTTP metadata, ranged requests, validators |
| `internal/storage` | Files, manifests, checkpoints, locking |
| `internal/progress` | Event definitions and progress snapshots |
| `internal/config` | Defaults and optional configuration |

The current checkout is smaller; see [development](development.md#current-checkout).

## Dependency boundaries

Prefer Go's standard library: `net/http`, `context`, `os`, `encoding/json`, `sync`, and `time`. A small CLI library is optional. A database is unnecessary for v0.1.

The transfer engine should not import terminal rendering or daemon code. Storage should not depend on CLI flags. Convert flags into a request before entering the engine. Keep range arithmetic independently testable from network and disk behavior.

Avoid a generic task framework, plugin system, or public SDK before the first transfer use case works. Concrete APIs can follow the implementation rather than preceding it.

## Transfer data

The conceptual request includes the source URL, destination, worker preference, retry policy, optional cap, and resume/overwrite intent. These are inputs to validate, not frozen Go fields.

The coordinator needs reliable representation metadata, the selected mode, interval ownership, completed coverage, and cancellation state. Persist only the information recovery can validate. Keep live counters distinct from durable checkpoints.

Snapshots should distinguish known and unknown totals and expose enough information for bytes, speed, ETA, and active workers. A progress subscriber must not decide whether a transfer is complete. Terminal success follows verification and final publication.

`cli/events.go` adapts engine snapshots/results into the [version-1 JSON contract](cli.md#json-output). `cli/json.go` cancels the engine on output failure and emits the terminal result only after `Download` returns with worker/storage cleanup complete. JSON output owns duplicated pipe/socket descriptors through `cli/output.go` and `output_unix.go`; writes are synchronous, bounded by deadlines where supported, and never create an event backlog. Additional subscriptions and retry counters remain [open](decisions.md#command-and-output-decisions), subject to a concrete future requirement.

`cli/text.go` applies the same writer to one-shot replies/diagnostics and text downloads. Progress owns its writer only during the transfer, then releases it before terminal reporting; stdout/stderr handles do not overlap when both refer to the same pipe. `render.go` escapes nonprinting text, and `terminal_unix.go` detects terminal geometry for clipped ANSI frames. Detected terminals also use the owned, cancellable output path.

## Ownership and shutdown

One process owns a transfer's writable artifacts. Within that process, each worker owns one interval at a time. Shared metadata needs coordinated updates even when data writes target disjoint offsets.

A context from the CLI signal handler flows through the application, requests, workers, limiter, and retry waits. Shutdown waits for writes to stop before saving a final safe checkpoint or closing shared files.

See [concurrency](concurrency.md) and [resume and storage](resume-and-storage.md) for the invariants behind these boundaries.

## CLI and daemon boundaries

```mermaid
flowchart TD
    CLI[Queue CLI] --> IPC[Local IPC and daemon API]
    IPC --> Queue[Persistent queue manager]
    Queue --> App[Transfer application service]
    Direct[Standalone download CLI] --> App
    App --> Engine[Download engine]
```

The [accepted v0.2 contract](daemon-and-queue.md) uses `internal/queue` for durable state and scheduling and `internal/daemon` for Linux local control/lifecycle. Both are implemented, including composed shared budgets and CLI adapters. Standalone `transfer.Download` and the retained `Budget.Download` runner share the same engine, pure validation, and body accounting. The engine never imports daemon or CLI code.

The queue coordinator serializes durable mutations and owns transfer contexts. Persist running intent before engine calls, and record outcomes after the engine joins workers and releases storage. Safe status records omit stored source URLs; progress is coalesced and never blocks the engine on a control client. Existing clients use the safe polling API; future consumers need a separate contract.

Initial control uses a private Linux Unix stream socket with peer-UID checks and separate state/runtime locks. `daemon start` runs in the foreground without automatic startup or service installation. The standalone commands remain independent. Other native daemon transports require their own qualification.

## Planned web application

The owner selected Vue 3 + TypeScript for the local web app and Vue Flow for its Visual Grabber editor. The [product roadmap](roadmap.md) is feature-based: build the local application first, with browser-extension and hosted-instance targeting explicitly later. These components are unimplemented; the diagram describes responsibility boundaries rather than a required process or package per box.

```mermaid
flowchart LR
    UI[Vue web UI] --> App[Go application API]
    Extension[Later Firefox extension] --> Bridge[Later local bridge or remote HTTPS]
    Bridge --> App
    App --> IPC[Existing private daemon IPC]
    CLI[Existing queue CLI] --> IPC
    IPC --> Daemon[Download daemon and engine]
    App --> Data[Profiles, rules, library and history]
    App --> Runs[Schedules and workflow runs]
    Runs --> Browser[Optional Playwright worker]
    Browser -->|Download candidates| App
```

The [W1 local contract](local-web.md) selects a separate explicit foreground Go web process serving built Vue/Vite assets and a bearer-authenticated, same-origin loopback API. It calls the existing private daemon IPC; web/tab shutdown does not stop daemon work. Standalone Go builds remain independent of the frontend build. Hosted access adds its own transport/identity contract later.

The [configuration contract](configuration.md) selects one XDG JSON authority for global/profile overrides, shared by CLI and web through short transactions and content-token checks. Its pure config model, resolution, and scoped edits are implemented. The planned separate version-2 app ledger stores instance binding and immutable URL-free submissions, not editable defaults. Use a distinct web-lifetime lock. Persist a submission before one daemon Add, never replay it, and poll safe IPC views. Unprofiled CLI jobs remain Unassigned; profiled CLI admission will reuse the binding service. The [ordered implementation](local-web.md#delivery-order) next adds strict config encoding/loading and revises the earlier app model before filesystem stores and HTTP controls. Runtime integration remains unimplemented.

Vue Flow supplies editor interaction. Versioned workflow definitions, run state, cancellation, retries, loop/subflow bounds, and debugging belong to the execution model. An optional Playwright worker handles browser sessions and must qualify the handoff of public, authenticated/expiring, and browser-managed downloads separately. Neither frontend libraries nor browser tooling establish the Go engine's integrity guarantees for a new download path.

Keep instance location, profile/workspace identity, workflow versions, execution runs, download jobs, and library entries distinct. Share their stable references across rules, schedules, history, and import/export. W1 profiles and submission metadata are specified; subsequent schemas, richer telemetry, priority/reordering APIs, aggregate profile limits, and browser credentials remain [open contracts](decisions.md#web-application-direction). These plans do not install dependencies or change the daemon wire format.
