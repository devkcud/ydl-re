# Architecture

[Documentation index](README.md)

This is the proposed architecture. None of the packages or APIs below exists yet.

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
| Progress | Events and snapshots suitable for terminal and later daemon/TUI consumers |

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

The exact event schema and subscriber behavior remain [open](decisions.md#command-and-output-decisions).

## Ownership and shutdown

One process owns a transfer's writable artifacts. Within that process, each worker owns one interval at a time. Shared metadata needs coordinated updates even when data writes target disjoint offsets.

A context from the CLI signal handler flows through the application, requests, workers, limiter, and retry waits. Shutdown waits for writes to stop before saving a final safe checkpoint or closing shared files.

See [concurrency](concurrency.md) and [resume and storage](resume-and-storage.md) for the invariants behind these boundaries.

## Future front ends

```mermaid
flowchart TD
    CLI[Queue CLI] --> IPC[Local IPC and daemon API]
    TUI[TUI] --> IPC
    IPC --> Queue[Persistent queue manager]
    Queue --> App[Transfer application service]
    Direct[Standalone download CLI] --> App
    App --> Engine[Download engine]
```

The daemon should reuse the same application service. The standalone command must remain useful without it. A TUI consumes snapshots and events rather than reaching into workers.

Unix domain sockets are a suggested local transport on Linux/macOS; Windows needs an appropriate equivalent. No fixed port, IPC schema, REST endpoints, or authentication protocol is committed. Those decisions belong to v0.2 design work.
