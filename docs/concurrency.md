# Concurrency and retries

[Documentation index](README.md)

The current engine uses one sequential stream with cancellation, progress, and validated suffix resume. The range planner is implemented; parallel range workers, retries, and the limiter are not implemented yet.

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

The planner tests exact coverage, every completion subset of small fixtures, malformed coverage, and offsets through the maximum signed 64-bit length. The plan bounds metadata, not worker concurrency; the worker ceiling, queue, and buffer policy remain open. Very large files use larger chunks and may need to re-fetch more uncheckpointed data after interruption.

## Bounded workers

The proposed default is four workers, bounded by the request and object size. A worker owns one interval at a time and writes only within it, using offset writes such as `WriteAt` when appropriate.

Bound active requests, buffers, queued work, and retry activity. A retry must go through the same scheduler limits. More connections may reduce performance or trigger server limits; maximum throughput is secondary to correctness.

Data writes can target disjoint regions concurrently. Updates to coverage, counters, failure state, and checkpoints still need synchronization. Do not let a failed interval re-enter the queue while another worker still owns it.

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

The current CLI renders snapshots at most every 200 ms while data arrives, plus a final snapshot. Percentage includes validated saved bytes; average speed counts only bytes received during the current attempt, starting after local validation. Retry accounting, a durable event schema, and slow-subscriber behavior still need a contract for later consumers. Coverage and final verification remain the authority for completion.

## Deferred tuning

Static scheduling comes first. Adaptive chunk sizing, dynamic redistribution, round-robin heuristics, and multi-machine work distribution need measurements and separate scope decisions.

Validate this design with the [testing matrix](testing.md), especially retry, cancellation, throttling, overlap, and race scenarios.
