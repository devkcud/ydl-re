# Project status

Updated: October 11, 2026.

## Current state

Milestones A and B are implemented. Milestone C supports fresh and resumed parallel HTTP(S) downloads, exclusive offset writes, verified chunk checkpoints, and connection limits. Sequential resume, inspect, progress, cancellation, overwrite protection, and inactivity timeouts remain supported.

Milestone C remains active: retries and parallel crash-recovery evidence are unfinished. Bandwidth caps, JSON events, configuration loading, and release tooling also remain. Go 1.27 and the standard library are still the only runtime development requirements.

## Handoff

- `internal/ranges` plans at most 256 fixed half-open chunks, defaulting to at least four MiB each. It validates sorted whole-chunk coverage, subtracts completed work, and converts HTTP endpoints without overflow. Empty plans contain no tasks. Large files use larger chunks and can lose more uncheckpointed work on interruption.
- `internal/storage/manifest.go` and `range_manifest.go` read distinct strict schemas. Version 1 keeps `completed = n` as prefix `[0, n)` with its digest. Version 2 stores `chunk_size` and `completed_ranges` with per-chunk digests; saved-byte totals are derived, never resume offsets. No implicit schema conversion occurs. URL identities remain hashed.
- `range_writer.go` adds `ClaimRange` and bounded `RangeWriter` writes. Active/completed chunks cannot be claimed again; stale writers cannot write after release. `Commit` serializes sync/hash/checkpoint publication and releases the claim only after success. Call it only after validating clean HTTP completion; `Close` releases failed attempts without acknowledging them. File close/publication refuses active writers.
- `range_checkpoint.go` retains explicit v2 initialization/recovery and verified checkpoints. `FinalizeContext` checks cancellation during range hashes and before publication; OS sync remains non-interruptible. Existing version-1 APIs and ready recovery stay compatible.
- Version-1 checkpoint timing and resume behavior remain unchanged: sync before saving after four MiB/one second and on normal interruption; truncate unconfirmed tails only after a compatible response. Range checkpoints acknowledge whole chunks only, and retain initialized state even with zero saved bytes.
- Stable `.yoinkdl.lock` inodes hold nonblocking advisory locks. Process death releases ownership, so do not unlink lock files for stale-owner recovery. Orphan temporary manifests are ignored.
- `internal/transfer/resume.go` validates echoed strong ETags, identity encoding, source/effective URL fingerprints, exact range bounds, and lengths before reuse. Changed/unsupported responses preserve existing artifacts. A 416 is re-probed and refused.
- `internal/transfer/workers.go` uses at most 32 workers, at most 256 queued chunks, one 64 KiB payload buffer per worker, and a progress channel bounded by worker count. Workers validate headers before claiming offsets and require exact body length/clean EOF before committing. Errors cancel peers before claim cleanup; the coordinator drains events and joins all workers before returning. Progress callbacks run only on the coordinator.
- `--connections`/`-c` accepts decimal digits from 1 through 32 (default four; leading zeros allowed). One streams fresh files directly or resumes stored range tasks with one worker. Higher limits reduce worker count to missing chunks; saved chunk size is retained. Version-1 checkpoints always resume sequentially. The library uses zero as its default setting.
- `internal/transfer/representation.go` captures identity by value and binds every response to the selected version, effective URL, length, encoding, and assigned interval. Its exact range-bound validator is shared with version-1 resume. Worker 416 responses are re-probed and refused; active range state never falls back destructively.
- `internal/transfer/probe.go` proves fresh range capability with `GET bytes=0-0`, strict headers, and exactly one byte with clean EOF. It returns a selected identity or an open full-response stream. The coordinator must consume/close that stream and use its own metadata. Ignored ranges reuse the untouched 200 body; ineligible valid 206/416 responses get one fresh full GET after discarding probe bytes. Invalid probes fail. No storage writes or saved-state fallback occur.
- `Download` invokes the fresh probe after output ownership, initializes version 2 for ranges, or consumes a returned full stream using version-1 storage. `range_resume.go` separately validates version-2 checkpoints before activating writers: conditional byte-zero GET, saved ETag/effective URL/total/encoding, exact bounds, and clean one-byte completion. Empty range state uses `If-Match` with a known empty 200 body. Probes never count toward coverage/progress.
- Refused/timed-out/canceled resume validation preserves partial bytes and metadata. There is no saved-state fallback. Fully covered partial checkpoints still validate remotely; only ready checkpoints recover offline. Initial saved progress is reported before spawning workers so cancellation at that boundary starts no range work. Failed worker attempts remain whole missing chunks for the next invocation.
- Both schemas support verified offline `ready` recovery. `inspect` reads metadata without hashing or writer ownership. Progress includes received bytes, which may lead whole-chunk checkpoints; worker counts reach zero before final verification. Cancellation propagates through range hashes and prevents publication.
- The original design is unchanged. The spacing rule, handoffs after every implementation, focused local commits, and no-push rule remain in force.

## Validation

- `go test ./internal/transfer -run 'TestRangeResume|TestPartialRangeState' -count=1` and `go test -race ./internal/transfer ./internal/cli` passed. New coverage includes automatic/explicit/disjoint/empty/full/one-worker resumes, unchanged-artifact refusal, local corruption, source changes, timeouts, cancellation, and replacing interrupted chunks in full.
- The resume `make check` passed: formatting, vet, all tests, race checks, and build. The initial sandbox run could not open localhost listeners; approved outside-sandbox checks passed. Parallel process-crash/resume evidence is the next validation slice.
- Documentation audit passed: 20 Markdown files, 177 local links/anchors, 23 shell blocks, two JSON examples, one TOML example, fences, and whitespace. Both manifests passed checksum checks and built-CLI inspection; built help describes range resume. The original design is unchanged and its illustrative shell placeholders are excluded from runnable-example checks.
- Environment: Go 1.27.2 on Linux/amd64. HTTP tests need localhost listeners outside the sandbox. macOS native checks, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Add forced-process-termination evidence for parallel resume: retain disjoint checkpoints and uncertain bytes, restart in a new process, and verify exact final content and completed-chunk reuse.
2. Add bounded retry/backoff and settle the retry flag contract. Test failed chunk replacement, attempt limits, status classification, and cancellation during backoff.
3. Complete focused/full gates for retries and close C before hardening in D.

## Open decisions and blockers

No external blocker remains. Partial reuse requires a known length, identity encoding, a matching strong ETag, and exact URL fingerprints. Refreshed signed URLs, weak/unversioned/encoded partials, and old partials without manifests are refused rather than silently restarted.

A full v0.1 release is not complete. Retry policy, license/distribution, native platform qualification, and remaining interfaces are tracked in [the decision register](docs/decisions.md). Split focused commits and never push.
