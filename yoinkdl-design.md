# YoinkDL

**Engineering design & implementation plan**  
**Status:** Proposed design · October 2026  
**Language:** Go  
**CLI:** `yoinkdl`  
**Tagline:** *The download manager that yoinks.*

## 1. Summary

YoinkDL is an open-source, local-first download manager for reliable HTTP(S) transfers. It starts as a standalone command-line program that can download, pause by interruption, resume safely, and report progress. Subsequent versions add a persistent local daemon, queue management, a TUI, and optional automation.

The first release should be useful *without* a daemon, database server, web dashboard, scripting engine, or distributed cluster.

### Design principles

1. **Correctness over maximum throughput.** Never silently return a corrupted or incomplete file.
2. **Interruption is normal.** A terminated process, dropped connection, or reboot should not force a restart when the server supports safe resumption.
3. **Graceful fallback.** If range requests are not trustworthy, use a single streaming transfer.
4. **One engine, multiple front ends.** CLI now; daemon and TUI later.
5. **Small operational footprint.** Local state, bounded concurrency, and no mandatory services.
6. **Simple configuration.** Sensible defaults; advanced tuning is opt-in.

## 2. Release boundaries

| Release | Deliverable | Explicitly not required |
|---|---|---|
| **v0.1** | CLI; HTTP(S); direct URL → file; range-aware multi-part download; resumable local state; retries; progress; abort handling; integrity and completion checks | Daemon, TUI, queues, workflows |
| **v0.2** | Persistent queue and local daemon; add/list/pause/resume/cancel; bandwidth limits; concurrency scheduling | Web dashboard, plugins |
| **v0.3** | TUI; named profiles; richer inspection and history; optional completion hooks | Arbitrary workflow DSL |
| **Later** | Web UI, SFTP, torrents, Metalink, remote nodes, LAN swarm, scheduling, sandboxed workflows | No commitment until core is stable |

**Non-goals for v0.1:** HLS/DASH streaming, protected-content bypassing, browser automation, scraping, captcha solving, DRM circumvention, FTP/SFTP, torrents, LAN P2P, peer discovery, plugins, scripting runtimes, multi-machine transfers, or automatic unpacking.

## 3. Product behavior and CLI

The v0.1 commands should be deliberately few:

```sh
# Download to the current directory
 yoinkdl download https://example.org/archive.zip

# Explicit filename and directory
 yoinkdl download https://example.org/archive.zip --output ./downloads/archive.zip

# Choose how many ranges to fetch in parallel
 yoinkdl download https://example.org/archive.zip --connections 4

# Cap aggregate transfer speed (decimal MB/s)
 yoinkdl download https://example.org/archive.zip --limit 5MB/s

# Continue an interrupted transfer using the same output path
 yoinkdl download https://example.org/archive.zip --output ./downloads/archive.zip --resume

# Show locally stored transfer metadata
 yoinkdl inspect ./downloads/archive.zip

# Display installed version
 yoinkdl version
```

The parser may accept `yoinkdl <url>` as a convenience alias *later*, but the canonical, scriptable interface stays `yoinkdl download <url>` as in the original project.

### Suggested v0.1 flags

| Flag | Behavior | Default |
|---|---|---|
| `--output, -o` | Explicit final path | Derived from safe URL path; fallback `download` |
| `--connections, -c` | Desired parallel range workers | `4` |
| `--limit` | Aggregate transfer speed cap | Unlimited |
| `--resume` | Resume matching existing transfer; fail rather than overwrite incompatible partial state | Enabled when compatible partial state exists; explicit flag supported |
| `--overwrite` | Replace an already completed output, deliberately | Off |
| `--timeout` | Per-request/idle timeout policy | Reasonable defaults |
| `--retries` | Retries for transient failures | `3` per range |
| `--json` | Machine-readable terminal result/events | Off |
| `--quiet` | Errors and final output only | Off |

Avoid a broad `--range` override in v0.1: it complicates resume semantics and can be added when a clear use case exists. Keep named profiles and hook flags for later releases.

### Terminal UX

A single transfer gets a compact progress line showing bytes, percentage (when total length known), instantaneous/smoothed rate, ETA (when meaningful), and active workers. Unknown-length responses show bytes transferred and speed, without fictional percentage or ETA. On successful completion, print the final path. On failure, print a clear reason and whether a partial transfer was preserved.

Exit status: `0` success; nonzero failure. Reserve specific exit codes only if scripting needs justify them.

## 4. Architecture

```text
CLI / command parser
        |
        v
Application service  ------------------------> Progress events
        |
        v
Transfer coordinator
   |       |        |
   |       |        +--> State store (manifest + checkpoints)
   |       +------------> HTTP probe / response validation
   +--------------------> Scheduler / bounded range workers
                                |
                                v
                          File writer
                                |
                                v
                     Partial file -> final file
```

**Separation of concerns**

- **CLI:** argument parsing, error presentation, progress UI, cancellation signals.
- **Application service:** validates input; constructs download request; orchestrates the transfer lifecycle.
- **HTTP transport:** probing, redirects, streaming, response/validator handling, timeouts.
- **Planner:** chooses one stream or a set of disjoint byte ranges.
- **Scheduler:** caps requests and worker count; handles retries and cancellations.
- **Storage:** partial file, immutable transfer identity, validated checkpoints, atomic state writes.
- **Events:** progress/completion/failure events consumed by CLI and, later, daemon/TUI.

Prefer the standard library (`net/http`, `context`, `os`, `encoding/json`, `sync`, `time`) for the transfer engine. A small CLI library is optional. Don't add a database to v0.1.

### Suggested Go layout

```text
yoinkdl/
├── cmd/yoinkdl/main.go
├── internal/
│   ├── app/              # use cases and lifecycle
│   ├── transfer/         # coordinator, planner, workers
│   ├── httpx/            # probes, ranged GETs, validators
│   ├── storage/          # partial files and manifests
│   ├── progress/         # event types and snapshots
│   └── config/           # defaults + optional config
├── test/
│   ├── integration/      # local HTTP test servers
│   └── fixtures/
├── go.mod
├── README.md
└── LICENSE
```

A package should be added only when it has a responsibility distinct enough to deserve one. Resist designing a generalized task framework at the start.

## 5. Download lifecycle

```text
CREATED
   |
   v
PROBING --(fatal response)--> FAILED
   |
   v
PLANNING
   |
   v
DOWNLOADING <----> RETRYING
   |  ^                |
   |  +----------------+
   |
   +--(SIGINT/error)--> INTERRUPTED / FAILED (partial state retained)
   |
   v
VERIFYING
   |
   v
FINALIZING
   |
   v
COMPLETED
```

1. **Validate input:** accept HTTP/HTTPS only; enforce output path/overwrite policy; sanitize derived names.
2. **Probe:** optionally use `HEAD`, but do not rely on its availability or correctness. A ranged `GET` can establish capability when necessary. Track effective URL, content length if reliable, `ETag`, and `Last-Modified`.
3. **Select mode:** multi-part only if byte ranges are demonstrably supported, total length is reliable, and the response is identity encoded. Otherwise stream through one connection.
4. **Initialize storage:** create a unique sidecar manifest and same-filesystem partial data file. Reserve known length if useful, but do not treat allocation as proof of downloaded bytes.
5. **Transfer:** fetch byte ranges with bounded workers. Write only their assigned file intervals (e.g. `WriteAt`); record verified completed intervals.
6. **Checkpoint:** periodically persist manifest via write-temp → sync → rename; checkpoint acknowledged progress conservatively so restart never trusts undurable partial bytes.
7. **Verify:** ensure all expected bytes are present and ranges cover the whole known-length object exactly once; validate supplied user checksum if enabled in a later release. For unknown length, require clean successful EOF and transport completion.
8. **Finalize:** sync and close the partial file, atomically rename to the final path on the same filesystem, and remove transfer metadata when safe. Never expose a partial as a completed output.

### Correct HTTP handling

- `Accept-Ranges: bytes` **alone** is not proof. A ranged request must return **`206 Partial Content`** with a matching `Content-Range` and expected payload boundaries. A `200 OK` in response to a range request is not valid range output.
- `416 Range Not Satisfiable` requires re-probing; do not assume it signifies completion.
- Use `Accept-Encoding: identity` and disable transparent content decompression for byte-range transfers. Transfer ranges apply to the actual representation's bytes.
- Honor `ETag`/`Last-Modified` validators and use `If-Range` where applicable; if content changed or resume identity cannot be established confidently, discard/restart only with safe, explicit policy—never splice mismatched versions.
- Preserve necessary redirect behavior while considering changed effective URLs and credentials. Never forward sensitive headers indiscriminately across origins.
- A successful HTTP status does not establish download completeness: check byte count, exact range bounds, reader errors, and final coverage.
- Unknown-size, compressed, or non-range-compatible servers get a single sequential stream.
- Bound total concurrent requests and buffer memory. More connections aren't always faster; cap them and avoid tiny ranges.
- Timeouts must distinguish slow transfers from dead connections. One global short client timeout should not kill legitimate long downloads.

## 6. Resume and state model

Use a manifest beside the partial file in v0.1, making recovery discoverable without a daemon. Example names:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
```

Illustrative schema (not frozen public API):

```json
{
  "schema_version": 1,
  "source_url": "https://example.org/archive.zip",
  "effective_url": "https://cdn.example.org/archive.zip",
  "destination": "/home/user/Downloads/archive.zip",
  "length": 104857600,
  "etag": "\"example-validator\"",
  "last_modified": null,
  "mode": "ranged",
  "completed_ranges": [[0, 16777215], [33554432, 50331647]],
  "state": "interrupted"
}
```

The ranges above use **inclusive endpoints**; centralize that convention or, preferably, use internal half-open intervals `[start, end)` and convert only when constructing HTTP `Range` headers. The manifest shown is illustrative and can use half-open intervals in the real schema to eliminate off-by-one ambiguity.

**Key invariant:** resumed data must match both the intended resource version and the recorded completed byte intervals. A changed server object, inconsistent manifest, incompatible output path, or unusable partial file must not be silently resumed.

To avoid corruption after a crash, either flush completed intervals before committing them to the manifest or re-fetch conservatively after restart. Treat manifest writes as journal-like checkpoints, not an exact byte-by-byte history.

For v0.1, keep one process writing a transfer at a time; use an exclusive lock/lockfile with defensible stale-lock recovery. Do not let two processes modify the same partial file.

## 7. Concurrency, retries, and limits

- **Planning:** fixed ranges from a reliable known length; split sufficiently large files into several non-overlapping tasks. Start with static scheduling; adaptive chunk sizing is future work.
- **Workers:** default four, bounded by requested count and object size. Each worker owns one interval at a time; a failed interval re-enters the scheduler only if retryable.
- **Retries:** exponential backoff with bounded jitter and maximum attempts. Retry selected network errors, timeouts, `408`, `429`, and `5xx` as appropriate; honor reasonable `Retry-After` values. Do not blindly retry all `4xx`.
- **Throttling:** optional aggregate token bucket across all workers, not a separate unlimited budget per connection.
- **Cancellation:** `context.Context` propagates from signal handler to all workers; persist safe checkpoints on normal interrupt.
- **Progress:** sum only committed or reliably received bytes without double-counting retries. Speed and ETA are advisory, not integrity evidence.

A `GET` must not be retried into the same output interval without ensuring that partial failed writes are replaced consistently and the interval is not marked complete prematurely.

## 8. State and configuration paths

The old GoonDL design requested a single user-owned folder for simple cleanup. Preserve that spirit with a new name:

```text
~/.config/yoinkdl/
├── config.toml           # optional, sensible built-in defaults
├── state/                # reserved for daemon / persistent queue later
├── logs/                 # reserved for later
└── profiles/             # reserved for later
```

Sidecar manifests remain next to user-selected download destinations. This is an **intentional exception** to centralizing all configuration/state in one directory: it keeps CLI-only resume portable and self-discovering. Provide a cleanup command later if stale partial artifacts become a problem.

Suggested initial configuration:

```toml
[download]
connections = 4
retries = 3
resume = true

[network]
# Optional overall download cap; absent means unlimited.
# limit = "5MB/s"
```

For the original project's custom-size math parser, keep it as a separate, optional utility; v0.1 only requires reliable parsing of straightforward byte/speed units. Define decimal (`MB`) versus binary (`MiB`) explicitly.

## 9. Security and file safety

- Never run downloaded files or completion scripts automatically.
- Do not overwrite an existing completed file without `--overwrite`.
- Sanitize filenames from URLs; prefer explicit `--output` over server-supplied `Content-Disposition` for v0.1. Guard against path traversal and special filenames.
- Respect system proxy and certificate verification defaults; do not include an insecure-TLS option without an explicit use case.
- Limit redirects and do not leak `Authorization` or cookies to unrelated origins.
- Avoid printing credentials or URL tokens in verbose logs/manifests; consider redacted display and restricted file permissions.
- Check available disk space when possible, and report `ENOSPC` without deleting otherwise valid partial data.
- Use safe permissions for state files and exclusive creation/locking where appropriate.
- For any future daemon that accepts remote URLs, explicitly address SSRF, loopback/private destinations, permissions, and authentication before exposing an API beyond the local machine.

## 10. Testing strategy

Use `httptest.Server` and deterministic fixtures rather than public internet URLs for correctness tests.

**Required scenarios:**

1. Basic `200` streaming download; exact output match.
2. Correct `206` range support; interval coverage; exact output match.
3. Server ignores `Range` and returns `200`; fallback to sequential safely.
4. Invalid `Content-Range`, short responses, wrong lengths, or midstream EOF; never report success.
5. Server changes `ETag` between runs; resume is rejected/restarted safely.
6. Interrupt and relaunch; correct resume of already committed intervals.
7. Connection drop during a range; only safe bytes survive/retry.
8. Rate limiting across multiple workers.
9. Unknown-length transfer; completion only on normal end of response.
10. Destination collision, permissions errors, no disk space, corrupt manifest, simultaneous writers.
11. `429`/`503` retries with backoff; permanent `404` failure.
12. Race detector and leak checks (`go test -race ./...`).

A good end-to-end test computes SHA-256 of the generated fixture and the downloaded result; this is a **test assertion** even before a user-facing `--checksum` flag is implemented.

## 11. Concrete implementation sequence

**Milestone A — Reliable single stream**

- CLI `download` and `version`.
- Stream into `.part`, print progress, handle cancellation.
- Fail cleanly on bad HTTP statuses, partial responses, and destination conflicts.
- Atomic completion, deterministic local-server integration tests.

**Milestone B — Persistence and resume**

- Transfer manifest and checkpoint format.
- Server metadata probing and validator logic.
- Safe sequential resumption when supported; safe restart when not.
- `inspect` command and crash/restart tests.

**Milestone C — Multi-range engine**

- Plan disjoint chunks; bounded worker pool; concurrent offset writes.
- Validate every response's range and length.
- Retry/backoff; single-stream fallback.
- Run fault-injection tests and race detector.

**Milestone D — v0.1 hardening**

- Aggregate bandwidth cap, JSON output, polish UX and exit behavior.
- Security review, cross-platform build checks (Linux/macOS/Windows).
- README install/use examples; release binaries and changelog.

Only tag **v0.1.0** when these acceptance criteria pass: a direct download works; a supporting server resumes across process termination; range downloads never silently mix content; wrong lengths are detected; files are finalized atomically; simultaneous writers cannot corrupt state; integration and race tests pass.

## 12. Later architecture: daemon, queue, TUI

Do not make every v0.1 download require a daemon. When ready, add a long-running process that calls the **same transfer application service** as the CLI.

```text
       yoinkdl CLI       yoinkdl TUI
            \              /
             \            /
        Local IPC / daemon API
                  |
             Queue manager
                  |
           Transfer application
                  |
             Download engine
```

Suggested future commands:

```sh
 yoinkdl daemon start
 yoinkdl daemon stop
 yoinkdl add https://example.org/archive.zip
 yoinkdl list
 yoinkdl pause <id>
 yoinkdl resume <id>
 yoinkdl cancel <id>
 yoinkdl tui
```

The old `summon` command is a fun historical alias; use `daemon start` as the primary discoverable interface, possibly keeping `summon` as a hidden alias. Avoid promising a fixed localhost port like the old `5993` unless an HTTP API is needed. On Linux/macOS, a Unix domain socket is a reasonable default; use an appropriate local IPC equivalent on Windows.

The v0.2 queue persists transfer records and exposes lifecycle operations. Pausing means canceling active request contexts and checkpointing safely, not freezing goroutines arbitrarily. A TUI should subscribe to snapshots/events rather than access worker internals.

## 13. Deferred original concepts

Keep the history, not the implementation burden:

| Original GoonDL concept | Current decision |
|---|---|
| Adaptive chunk allocation / round-robin workers | Start with bounded static ranges; reconsider from benchmarks |
| `.goondl-meta` / custom extension ecosystem | Use versioned `.yoinkdl.json` first; avoid custom DSL/file formats until justified |
| `~/.config/goondl` single-root organization | Renamed to `~/.config/yoinkdl`, with nearby transfer sidecars |
| Profile/hook system (`goonprofile`, `goonhook`, etc.) | v0.3+, use basic declarative formats first |
| JavaScript/Lua/WASM workflows | Unscheduled; significant runtime/security complexity |
| HTMX monitoring dashboard | Unscheduled, optional interface to same daemon API |
| Swarm transfers, LAN P2P, SCP/SSH | Separate future protocol/transport projects |
| Browser automation and protected download handling | Out of scope |
| Paranoia Mode, voice notifications, mobile UI | Not on the delivery roadmap |
| Arbitrary-precision size expression parser | Retain separately if useful, not a blocker |

## 14. Definition of success

YoinkDL v0.1 succeeds if a user can run `yoinkdl download <url>`, get a correct file, interrupt and recover a compatible transfer, understand failures, and trust the manager not to produce silent corruption.

**Core product promise:** *Yoink a file. Keep it intact. Pick up where you left off.*
