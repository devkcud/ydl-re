# Project status

Updated: October 10, 2026.

## Current state

Milestones A and B are implemented. Milestone C now supports fresh parallel HTTP(S) downloads, exclusive offset writes, verified chunk checkpoints, and connection limits. Sequential resume, inspect, progress, cancellation, overwrite protection, and inactivity timeouts remain supported.

Milestone C remains active: version-2 partial resume, retries, and parallel crash-recovery evidence are unfinished. Bandwidth caps, JSON events, configuration loading, and release tooling also remain. Go 1.27 and the standard library are still the only runtime development requirements.

## Handoff

- `internal/ranges` plans at most 256 fixed half-open chunks, defaulting to at least four MiB each. It validates sorted whole-chunk coverage, subtracts completed work, and converts HTTP endpoints without overflow. Empty plans contain no tasks. Large files use larger chunks and can lose more uncheckpointed work on interruption.
- `internal/storage/manifest.go` and `range_manifest.go` read distinct strict schemas. Version 1 keeps `completed = n` as prefix `[0, n)` with its digest. Version 2 stores `chunk_size` and `completed_ranges` with per-chunk digests; saved-byte totals are derived, never resume offsets. No implicit schema conversion occurs. URL identities remain hashed.
- `range_writer.go` adds `ClaimRange` and bounded `RangeWriter` writes. Active/completed chunks cannot be claimed again; stale writers cannot write after release. `Commit` serializes sync/hash/checkpoint publication and releases the claim only after success. Call it only after validating clean HTTP completion; `Close` releases failed attempts without acknowledging them. File close/publication refuses active writers.
- `range_checkpoint.go` retains explicit v2 initialization/recovery and verified checkpoints. `FinalizeContext` checks cancellation during range hashes and before publication; OS sync remains non-interruptible. Existing version-1 APIs and ready recovery stay compatible.
- Version-1 checkpoint timing and resume behavior remain unchanged: sync before saving after four MiB/one second and on normal interruption; truncate unconfirmed tails only after a compatible response. Range checkpoints acknowledge whole chunks only, and retain initialized state even with zero saved bytes.
- Stable `.yoinkdl.lock` inodes hold nonblocking advisory locks. Process death releases ownership, so do not unlink lock files for stale-owner recovery. Orphan temporary manifests are ignored.
- `internal/transfer/resume.go` validates echoed strong ETags, identity encoding, source/effective URL fingerprints, exact range bounds, and lengths before reuse. Changed/unsupported responses preserve existing artifacts. A 416 is re-probed and refused.
- `internal/transfer/workers.go` uses at most 32 workers, at most 256 queued chunks, one 64 KiB payload buffer per worker, and a progress channel bounded by worker count. Workers validate headers before claiming offsets and require exact body length/clean EOF before committing. Errors cancel peers before claim cleanup; the coordinator drains events and joins all workers before returning. Progress callbacks run only on the coordinator.
- `--connections`/`-c` accepts decimal digits from 1 through 32 (default four; leading zeros allowed). One uses a direct sequential stream; higher limits probe fresh transfers and reduce worker count to available chunks. Version-1 checkpoints always resume sequentially. The library uses zero as its default setting.
- `internal/transfer/representation.go` captures identity by value and binds every response to the selected version, effective URL, length, encoding, and assigned interval. Its exact range-bound validator is shared with version-1 resume. Worker 416 responses are re-probed and refused; active range state never falls back destructively.
- `internal/transfer/probe.go` proves fresh range capability with `GET bytes=0-0`, strict headers, and exactly one byte with clean EOF. It returns a selected identity or an open full-response stream. The coordinator must consume/close that stream and use its own metadata. Ignored ranges reuse the untouched 200 body; ineligible valid 206/416 responses get one fresh full GET after discarding probe bytes. Invalid probes fail. No storage writes or saved-state fallback occur.
- `Download` now invokes the probe after fresh output ownership. It initializes version 2 for range workers or consumes the returned full stream using version-1 storage. Probe bytes never count toward coverage/progress. Partial version-2 resume is still refused; use `--connections 1` from the start when current resume support is required.
- Both schemas support verified offline `ready` recovery. `inspect` reads metadata without hashing or writer ownership. Progress includes received bytes, which may lead whole-chunk checkpoints; worker counts reach zero before final verification. Cancellation propagates through range hashes and prevents publication.
- The original design is unchanged. The spacing rule, handoffs after every implementation, focused local commits, and no-push rule remain in force.

## Validation

- `go test -race ./internal/storage` and the storage `make check` passed for exclusive writers, serialized checkpoints, and context-aware finalization.
- `go test -race ./internal/transfer ./internal/cli ./cmd/yoinkdl` passed for the worker/CLI integration. `go test ./internal/transfer -run 'TestParallel|TestWorker|TestInvalidWorker' -count=1` passed after adding cancellation-before-publication and invalid-configuration tests. Exact fixture hashes, concurrency bounds, malformed/chunked bodies, peer shutdown, checkpoint preservation, and actual CLI-process downloads are covered.
- The final `make check` passed: formatting, vet, all tests, race checks, and build. Existing sequential process-crash resume and version-2 refusal/recovery tests pass; parallel partial resume/crash recovery remain unqualified.
- Documentation audit passed: 20 Markdown files, 176 local links/anchors, 23 shell blocks, two JSON examples, one TOML example, fences, and whitespace. Both manifest examples passed checksum checks and built-CLI inspection; the built help lists connection limits. The original design is unchanged and its illustrative shell placeholders are excluded from runnable-example syntax checks.
- Environment: Go 1.27.2 on Linux/amd64. HTTP tests need localhost listeners outside the sandbox. macOS native checks, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Implement version-2 partial resume: validate stored identity remotely before scheduling whole missing chunks; preserve saved state on mismatches, ignored ranges, and cancellation. Reuse the bounded pool without interpreting saved-byte totals as offsets.
2. Add bounded retry/backoff and settle the retry flag contract. Test failed chunk replacement, attempt limits, status classification, and cancellation during backoff.
3. Add parallel process-crash/resume evidence, complete focused/full gates, and close C before hardening in D.

## Open decisions and blockers

No external blocker remains. Partial reuse requires a known length, identity encoding, a matching strong ETag, and exact URL fingerprints. Refreshed signed URLs, weak/unversioned/encoded partials, and old partials without manifests are refused rather than silently restarted.

A full v0.1 release is not complete. Range resume, retry policy, license/distribution, native platform qualification, and remaining interfaces are tracked in [the decision register](docs/decisions.md). Split focused commits and never push.
