# Concurrency and retries

[Documentation index](README.md)

Fresh transfers use bounded range workers after a validated capability probe, with safe sequential fallback before range state begins. Sequential suffix resume remains supported; parallel resume, retries, and the limiter are not implemented yet.

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

The default is four workers, with `--connections`/`-c` accepting 1 through 32. Active workers are reduced to the number of planned chunks. One connection selects a direct sequential transfer without a capability probe. Existing version-1 checkpoints always use their sequential resume path.

`internal/transfer/workers.go` starts only the bounded worker pool and a completion waiter. The task queue contains at most 256 intervals; no goroutine is created per chunk. Each worker owns one 64 KiB payload buffer, so their total is at most two MiB, excluding HTTP transport and hashing overhead. A worker keeps its slot through response validation and checkpointing.

Each response must match the selected representation and assigned interval before acquiring a storage writer. Writes cannot leave that interval. Only exact body completion with clean EOF can commit the chunk. Storage serializes sync, checksum verification, and checkpoint publication; failed attempts release ownership without acknowledging bytes.

The first failure cancels peer requests and stops admitting tasks. The coordinator drains progress and waits for all workers and writer claims to finish before returning. A worker `416` is re-probed and refused. No automatic fallback or retry occurs once range state exists. Progress callbacks are serialized through a channel bounded by the worker count, applying backpressure without unbounded events.

## Retries

The design suggests exponential backoff with bounded jitter and a maximum attempt budget. The proposed `--retries` default is `3` per range; whether the numeric contract counts retries after the first attempt must be settled before publishing the flag.

| Failure | Intended treatment |
|---|---|
| Selected transient connection errors or timeouts | Retry the incomplete work within budget |
| `408`, `429`, selected `5xx` | Retry when appropriate; honor a reasonable `Retry-After` |
| Permanent `404` or another non-retryable request failure | Fail without a blind retry loop |
| Resource identity changes | Stop combining bytes; apply the safe restart/refusal policy |
| Invalid bounds, short data, or wrong length | Never acknowledge completion; classify safe recovery explicitly |
| Local permissions, disk-full, or state persistence failure | Report the local problem and preserve valid recovery data where possible |
| Cancellation | Stop; do not schedule another attempt |

The exact status allowlist, delays, jitter, retry budget for probes, and `Retry-After` ceiling remain open.

An interval retry must replace failed writes consistently. A conservative implementation re-fetches the entire incomplete interval, counting it once after successful validation. Restarting a request must not introduce duplicate coverage or leave unverified bytes marked complete.

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

The CLI renders snapshots at most every 200 ms while data arrives, plus a final snapshot. Percentage includes validated saved bytes; average speed counts only bytes received during the current attempt, excluding the capability probe. Worker counts reflect active chunk processing and reach zero before final verification. Retry accounting and a durable event schema remain future contracts. Coverage and final verification remain the authority for completion.

## Deferred tuning

Static scheduling comes first. Adaptive chunk sizing, dynamic redistribution, round-robin heuristics, and multi-machine work distribution need measurements and separate scope decisions.

Validate this design with the [testing matrix](testing.md), especially retry, cancellation, throttling, overlap, and race scenarios.
