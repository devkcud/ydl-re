# Roadmap

[Documentation index](README.md)

Every implementation milestone below is pending. The repository currently contains a proposed design and documentation; no release dates or completed features are implied.

## Delivery sequence

| Order | Milestone | Result |
|---|---|---|
| A | Reliable single stream | A useful foreground download command |
| B | Persistence and resume | Recovery from compatible local checkpoints |
| C | Multi-range engine | Safe bounded parallel transfers |
| D | v0.1 hardening | Validated UX, safety, platform behavior, and distribution |

Complete these in dependency order. A daemon or TUI should not delay the standalone engine.

## Milestone A — Reliable single stream

Deliver `download` and `version`, HTTP(S) streaming into a partial file, progress, cancellation, and verified final publication.

Reject bad HTTP responses, incomplete bodies, and destination conflicts. Use local-server integration tests with deterministic content.

Completion evidence: a direct download produces exactly the expected file; failed or interrupted transfers do not appear as completed output; publication obeys the chosen filesystem and overwrite policy.

Resolve the initial Go/module choices, output policy, and publication strategy before this milestone is called complete.

## Milestone B — Persistence and resume

Define the versioned manifest, interval convention, transfer identity, checkpoints, and exclusive ownership. Add server metadata probing, validator logic, safe sequential resumption when supported, and `inspect`.

Specify refusal or deliberate restart when state or server behavior is incompatible. Test normal interruption, process termination, malformed state, and recovery ordering.

Completion evidence: a supporting server can resume a terminated transfer without mixing versions or trusting undurable bytes. Competing processes cannot corrupt the same transfer.

## Milestone C — Multi-range engine

Plan disjoint chunks, bound worker concurrency, and write only assigned offsets. Validate every response's range, length, encoding, and compatible representation identity.

Add interval retries, bounded backoff, and safe sequential fallback. Exercise failures during concurrent transfer and run the race detector on meaningful paths.

Completion evidence: parallel and resumed outputs match fixtures exactly; retries and fallbacks do not duplicate coverage or splice different representations.

## Milestone D — v0.1 hardening

Add the aggregate per-transfer cap, JSON output, final terminal behavior, and explicit error/exit conventions. Complete security review and Linux/macOS/Windows build and native behavior checks where support is claimed.

Publish verified installation/use instructions, release binaries, and a changelog after choosing distribution and licensing. Update these docs from planned interfaces to observed behavior.

Completion evidence: all release acceptance criteria have recorded results, documented platform limits, and no unresolved integrity failure.

## v0.1.0 acceptance

Do not tag v0.1.0 until evidence shows that:

- A direct HTTP(S) download produces the expected file.
- A supporting server resumes a compatible transfer across process termination.
- Range transfers never silently combine different resource versions.
- Wrong lengths and incomplete responses are detected.
- Verified files are published atomically under the qualified platform policy.
- Simultaneous writers cannot corrupt data or metadata.
- Integration and race tests pass on the required environments.

The [testing matrix](testing.md#required-scenarios) expands these into concrete scenarios. Installation examples and platform support claims need real verification before publication.

## v0.2 — Daemon and persistent queue

Add a long-running local process that uses the same transfer application service. Persist queue records and support add, list, pause, resume, and cancel operations. Add concurrency scheduling and define bandwidth management across queued transfers.

Pause means canceling active contexts and checkpointing safely. It does not mean freezing goroutines. The standalone download command remains usable without a daemon.

The original release table also lists bandwidth limits here, while the detailed CLI and Milestone D place `--limit` in v0.1. Preserve the v0.1 per-transfer cap intent and resolve any broader daemon budget separately.

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

`daemon start` is the primary discoverable name. The historical `summon` alias is only a possible hidden compatibility convenience. There is no committed fixed port, HTTP API, or IPC protocol.

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
