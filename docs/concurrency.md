# Concurrency and retries

[Documentation index](README.md)

Fresh transfers use bounded range workers after a validated capability probe, with safe sequential fallback before range state begins. Parallel resume validates the saved representation before scheduling whole missing chunks. Transient requests and incomplete ranges have bounded retries; the aggregate limiter remains planned.

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

`--limit` is intended to constrain the whole standalone transfer. Use one shared token budget across its workers, not one full allowance per worker.

For example, `--connections 4 --limit 5MB/s` requests a combined cap of 5,000,000 bytes per second. It does not allow each worker that rate independently.

The proposed mechanism is a shared token bucket. Bucket capacity, burst allowance, timer granularity, and whether rate accounting includes probe/retry traffic still need decisions. Cancellation must interrupt limiter waits.

The v0.1 cap is not a specified global cap across separate CLI processes. Scheduling and bandwidth allocation across daemon-managed transfers belong to later design work.

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

The CLI renders snapshots at most every 200 ms while data arrives, plus a final snapshot. Percentage includes validated saved bytes; average speed tracks useful received bytes, excluding probes and bytes discarded before a replacement attempt. Progress can move backward when a chunk retries, but never counts its failed attempt twice. The coordinator reports initial saved progress before starting workers and honors cancellation there. Worker counts include slots in backoff and reach zero before final verification. A durable event schema remains future work; checkpoints and verification remain the authority for completion.

## Deferred tuning

Static scheduling comes first. Adaptive chunk sizing, dynamic redistribution, round-robin heuristics, and multi-machine work distribution need measurements and separate scope decisions.

Validate this design with the [testing matrix](testing.md), especially retry, cancellation, throttling, overlap, and race scenarios.
