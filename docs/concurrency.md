# Concurrency and retries

[Documentation index](README.md)

Fresh transfers use bounded range workers after a validated capability probe, with safe sequential fallback before range state begins. Parallel resume validates the saved representation before scheduling whole missing chunks. Transient requests and incomplete ranges have bounded retries. An optional aggregate body-rate cap shares one budget across the transfer.

## Range planning

`internal/ranges` plans fixed, disjoint intervals over a known nonnegative length. Multi-range downloading means multiple workers fetching intervals; it does not require a single HTTP request containing multiple ranges.

For a 4,096-byte example, four half-open intervals could be:

| Worker task | Internal interval | HTTP range | Bytes |
|---|---|---|---|
| 1 | `[0, 1024)` | `bytes=0-1023` | 1,024 |
| 2 | `[1024, 2048)` | `bytes=1024-2047` | 1,024 |
| 3 | `[2048, 3072)` | `bytes=2048-3071` | 1,024 |
| 4 | `[3072, 4096)` | `bytes=3072-4095` | 1,024 |

This is an arithmetic example. The production chunk size is the larger of four MiB and `ceil(length / 256)`, limiting a plan to 256 chunks. Small objects produce one task; zero-length objects produce none. The arithmetic helpers accept smaller positive chunk sizes for deterministic fixtures, while enforcing the same task limit.

Completed intervals must be sorted, disjoint, and match whole chunks in the original plan, including a shorter final chunk. Adjacent completed chunks remain separate. Duplicate, overlapping, partial, merged, reversed, or out-of-bounds intervals are refused. Resume subtracts these validated chunks without changing their boundaries. HTTP headers convert the exclusive end to `end - 1` only at the request boundary.

The planner tests exact coverage, every completion subset of small fixtures, malformed coverage, and offsets through the maximum signed 64-bit length. Very large files use larger chunks and may need to re-fetch more uncheckpointed data after interruption.

## Bounded workers

The default is four workers, with `--connections`/`-c` accepting 1 through 32. Active workers are reduced to the number of missing chunks. One connection selects a direct stream for fresh files or a single range worker for version-2 resume. Existing version-1 checkpoints always use their sequential resume path. Resuming retains the saved chunk size and boundaries.

`internal/transfer/workers.go` starts only the bounded worker pool and a completion waiter. The task queue contains at most 256 intervals; no goroutine is created per chunk. Each worker owns one 64 KiB payload buffer, so their total is at most two MiB, excluding HTTP transport and hashing overhead. A worker keeps its slot through response validation and checkpointing.

Each response must match the selected representation and assigned interval before acquiring a storage writer. Writes cannot leave that interval. Only exact body completion with clean EOF can commit the chunk. Storage serializes sync, checksum verification, and checkpoint publication; failed attempts release ownership without acknowledging bytes.

Retryable failures release the response and writer, then wait within the same worker slot before replacing the whole chunk. The first terminal failure or exhausted budget cancels peers and stops admitting tasks. The coordinator drains progress and joins all workers before returning. A worker `416` is re-probed once and refused. Active range state never falls back to a full stream. Progress callbacks use a channel bounded by worker count.

## Retries

`--retries` defaults to three additional attempts, with decimal values from zero through ten. Zero disables retries. Each chunk gets one shared budget for request failures and incomplete bodies, so header retries cannot multiply the chunk budget. The library's `Request.Retries` is optional: nil selects three; a pointer to zero disables retries.

Retry only HTTP `408`, `429`, `500`, `502`, `503`, and `504`, or selected transient connection failures: network timeouts, temporary DNS errors, resets, refused/aborted connections, broken pipes, and unexpected EOF. NXDOMAIN, TLS certificate/redirect policy failures, incompatible identity/bounds/encoding, malformed or oversized bodies, local storage errors, and cancellation are terminal. Short range bodies can retry by replacing the whole uncheckpointed chunk.

Exponential backoff starts at 125–250 ms, with equal jitter between half and all of the current delay and an eight-second cap. A single valid `Retry-After` delta or HTTP date can extend the wait up to 30 seconds; longer values stop the operation instead of retrying earlier than requested. Malformed or duplicate hints use normal backoff. All waits honor cancellation.

| Failure | Implemented treatment |
|---|---|
| Selected transient connection errors or timeouts | Retry the request/probe or whole incomplete range within its budget |
| `408`, `429`, `500`, `502`, `503`, `504` | Retry within budget and respect bounded `Retry-After` |
| Permanent `404` or another non-retryable request failure | Fail without a blind retry loop |
| Resource identity changes | Refuse without restart or further retries |
| Invalid bounds, encoding, declared length, malformed or oversized bodies | Refuse without acknowledging or retrying the chunk |
| Short range body or interrupted clean framing | Re-fetch the entire incomplete chunk; never acknowledge the failed attempt |
| Local permissions, disk-full, or state persistence failure | Report the local problem and preserve valid recovery data where possible |
| Cancellation | Stop; do not schedule another attempt |

Fresh probes and saved-range validation each have a separate bounded budget, including their small bodies. Full-stream request setup retries eligible failures before accepting a response. Sequential body interruption preserves its checkpoint for a later resume; it does not automatically restart a stream. A fresh fallback GET has its own setup budget and does not repeat the preceding probe. The diagnostic GET after a `416` remains a single request followed by refusal. Every attempt still uses the existing redirect limit.

An interval retry reuses the same immutable identity, bounds, worker slot, and buffer. The old writer is closed before waiting; the new attempt claims the chunk from its start. Only successful body completion and storage verification can commit coverage. Local storage and publication failures are never retried by the HTTP policy.

## Aggregate rate cap

`--limit` constrains body delivery for the whole standalone transfer through one shared token budget across its workers.

For example, `--connections 4 --limit 5MB/s` requests a combined cap of 5,000,000 bytes per second. It does not allow each worker that rate independently.

The shared token budget is implemented in `internal/transfer/limit.go` and applied to HTTP bodies by `body.go`. For a rate `R` in bytes per second, initial and maximum idle credit is `B = min(65536, ceil(R / 10))` bytes. This allows 100 ms of credit rounded up to a whole byte, capped at 64 KiB. Additional idle time does not build a larger burst.

Body reads are bounded by `B` and charged before delivery to the transfer code. All workers reserve against the same budget, including fresh/saved probe bytes and failed attempts. A failed/canceled read never refunds bytes already read. Saved file bytes do not consume credit. Waits honor cancellation and round up to nanoseconds. From budget creation, delivered body bytes cannot exceed `B + R * elapsed_seconds`. Late timer wakeups may cluster pending deliveries; scheduling or a slow server can lower throughput. Controlled-clock tests allow at most one microsecond of accumulated rounding; local HTTP timing checks allow five milliseconds for sampling.

This is application body pacing: headers, framing, TLS, and socket/HTTP read-ahead are outside the cap. Each active reader can hold at most one pending `B`-byte read while waiting. Body bytes read during redirects also count; unread rejected responses are closed without draining. Useful progress is separate and never supplies or refunds rate credit. Network inactivity timers cover only actual reads, excluding throttle waits. Ready-state recovery stays offline and consumes no budget. A resumed invocation gets a new budget and may use a different cap without changing its checkpoint.

The v0.1 cap does not coordinate separate CLI processes. The following v0.2 contract extends budgeting only for transfers owned by one daemon instance.

## Daemon-wide rate cap

This is accepted v0.2 design, not implemented behavior. `daemon start --limit R` creates one shared body budget for that process; omission means unlimited. Use the existing positive-integer rate grammar and bounds. The cap is fixed until restart and is not a persistent queue setting. `daemon set --parallel N` changes admissions without replacing or refilling the budget.

Each job retains its own optional `add --limit` setting. A read must satisfy both its transfer budget and the daemon budget; setting either to unlimited does not bypass the other. Reuse the transfer package's limiter/body integration through an optional shared budget handle; do not import daemon code into the engine or add a second HTTP implementation.

For each configured rate, retain `B = min(65536, ceil(R / 10))` and the existing accounting/timing tolerance. Bound each underlying read by the smallest applicable burst. Charge its actual returned byte count to every applicable budget before any throttle wait or cancellation return, including reads that also return an error. Wait until all reservations are eligible, using the latest deadline rather than adding independent waits. Never refund already-read bytes.

The daemon budget includes every managed transfer's probes, redirect drains, resumed network bytes, fallback, and failed/retried attempts. Saved file bytes and offline ready recovery consume no credit. Body delivery from daemon-budget creation stays within `B + R * elapsed_seconds`. Protocol overhead and socket read-ahead remain outside the cap, and each reader may hold one bounded pending read. Throttle waits stay cancellable and outside network inactivity timers.

Keep the shared budget alive across job completion, pause, cancel, failure, resume, and queue concurrency changes. Failed work and progress rollback cannot mint credit. Each explicit transfer attempt may create a fresh per-transfer budget, as the standalone command does; it cannot reset the daemon budget. A new daemon process starts a new bounded burst, without persisting token debt as transfer progress.

FIFO controls job admission, not equal bandwidth shares. Reservations use the existing serialized accounting; no minimum throughput, weighted fairness, or per-job share is promised. Separate daemons and standalone CLI processes remain independent. Dynamic rate changes and cross-process bandwidth coordination are outside v0.2.

## Cancellation

Propagate `context.Context` from the signal handler through requests, workers, backoff timers, and limiter waits. On a normal interrupt:

1. Stop admitting new work.
2. Cancel requests and waits.
3. Wait for writers to stop.
4. Persist only safely completed progress.
5. Close resources and report interruption with the state-preservation outcome.

A later daemon pause should use the same cancellation/checkpoint mechanism. Do not implement pause by arbitrarily freezing worker goroutines.

## Progress accounting

Track useful completed or reliably received bytes without double-counting retries. The display may lead the durable checkpoint, so resumed progress can be lower after a crash.

Show speed and ETA only as advisory estimates. Suppress total percentage and ETA when total length is unknown. Report the active worker count rather than implying the configured maximum is always in use.

The text CLI renders snapshots at most every 200 ms while data arrives, plus a final snapshot. Percentage includes validated saved bytes; average speed tracks useful received bytes, excluding probes and bytes discarded before a replacement attempt. Progress can move backward when a chunk retries, but never counts its failed attempt twice. The coordinator reports initial saved progress before starting workers and honors cancellation there. Worker counts include slots in backoff and reach zero before final verification. Checkpoints and verification remain the authority for completion.

`--json` emits advisory progress at the same interval and one terminal result after transfer cleanup. `--quiet` suppresses progress only. The CLI writes synchronously with bounded memory; a slow consumer applies backpressure, while deadline-capable output can time out or be interrupted. Output errors cancel the transfer and join its workers before reporting known preserved/published paths. The [JSON schema](cli.md#json-output) does not introduce a replay log or a general subscriber bus.

## Deferred tuning

Static scheduling comes first. Adaptive chunk sizing, dynamic redistribution, round-robin heuristics, and multi-machine work distribution need measurements and separate scope decisions.

Validate this design with the [testing matrix](testing.md), especially retry, cancellation, throttling, overlap, and race scenarios.
