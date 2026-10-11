# Project status

Updated: October 10, 2026.

## Current state

Milestones A and B are implemented and pass local Linux checks. The built CLI supports sequential HTTP(S) downloads, automatic compatible resume, explicit `--resume`, read-only `inspect`, progress, cancellation, overwrite protection, and network inactivity timeouts.

Milestone C is active. Bounded range planning and version-2 disjoint chunk persistence are implemented. Parallel HTTP transfers, workers, retries, bandwidth caps, JSON events, configuration loading, and release tooling remain unimplemented. Go 1.27 and the standard library are still the only runtime development requirements.

## Handoff

- `internal/ranges` plans at most 256 fixed half-open chunks, defaulting to at least four MiB each. It validates sorted whole-chunk coverage, subtracts completed work, and converts HTTP endpoints without overflow. Empty plans contain no tasks. Large files use larger chunks and can lose more uncheckpointed work on interruption.
- `internal/storage/manifest.go` and `range_manifest.go` read distinct strict schemas. Version 1 keeps `completed = n` as prefix `[0, n)` with its digest. Version 2 stores `chunk_size` and `completed_ranges` with per-chunk digests; saved-byte totals are derived, never resume offsets. No implicit schema conversion occurs. URL identities remain hashed.
- `range_checkpoint.go` provides `BeginRanges`, `ContinueRanges`, and `CheckpointRange`. The caller must validate complete HTTP responses and serialize checkpoint/lifecycle calls. Storage syncs and hashes each chunk before publishing metadata, preserves unacknowledged gaps, and verifies exact coverage plus a full-file hash before ready publication. Worker offset APIs and cancellation during final range verification remain to implement.
- Version-1 checkpoint timing and resume behavior remain unchanged: sync before saving after four MiB/one second and on normal interruption; truncate unconfirmed tails only after a compatible response. Range checkpoints acknowledge whole chunks only, and retain initialized state even with zero saved bytes.
- Stable `.yoinkdl.lock` inodes hold nonblocking advisory locks. Process death releases ownership, so do not unlink lock files for stale-owner recovery. Orphan temporary manifests are ignored.
- `internal/transfer/resume.go` validates echoed strong ETags, identity encoding, source/effective URL fingerprints, exact range bounds, and lengths before reuse. Changed/unsupported responses preserve existing artifacts. A 416 is re-probed and refused.
- Both schemas support verified offline `ready` recovery, including interrupted link/rename publication. `inspect` reads either schema without hashing or acquiring writer ownership. The current downloader creates version 1 and refuses version-2 partials before network access; it never sends an aggregate saved-byte count as a suffix offset.
- The original design is unchanged. The spacing rule, handoffs after every implementation, focused local commits, and no-push rule remain in force.

## Validation

- `go test ./internal/ranges` and `go test ./internal/storage ./internal/transfer ./internal/cli ./internal/ranges` passed. Coverage includes exact planning, maximum offsets, distinct schemas, strict nested JSON, 256-chunk capacity, out-of-order checkpoints, gap preservation, corrupt/missing bytes, cancellation, sync/publication failures, and ready recovery.
- `make check` passed for both implementation steps: formatting, vet, all tests, race checks, and build. Sandbox attempts could not write the Go cache; approved outside-sandbox runs passed.
- Existing sequential HTTP and process-death tests pass, including actual killed-writer suffix recovery. New tests prove partial range refusal before network access and offline ready publication. Range fixtures use direct deterministic offset writes; concurrent workers and parallel HTTP remain unqualified.
- Documentation audit passed: 20 Markdown files, 170 local links/anchors, 23 shell blocks, two JSON examples, one TOML example, fences, and whitespace. Both manifest examples passed checksum checks and built-CLI inspection. The original design is unchanged; its illustrative shell placeholders are excluded from runnable-example syntax checks.
- Environment: Go 1.27.2 on Linux/amd64. HTTP tests need localhost listeners outside the sandbox. macOS native checks, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Add capability probing and immutable representation identity for fresh parallel transfers, using the existing strict response validators and safe sequential fallback where applicable. Keep version-1 resume unchanged.
2. Implement bounded workers with exclusive chunk ownership and offset writes. Serialize range checkpoints, re-fetch whole missing chunks, propagate cancellation through verification, and wire version-2 resume only after remote validation. Keep destination protection guarantees.
3. Add bounded retry/backoff and define the connection/retry flag contracts. Test ignored ranges, changing versions, failed chunk replacement, cancellation, and concurrent completion.
4. Complete focused/full gates and process-recovery evidence for parallel transfers, update this handoff, and close C before hardening in D.

## Open decisions and blockers

No external blocker remains. Partial reuse requires a known length, identity encoding, a matching strong ETag, and exact URL fingerprints. Refreshed signed URLs, weak/unversioned/encoded partials, and old partials without manifests are refused rather than silently restarted.

A full v0.1 release is not complete. Worker/queue/buffer limits, retry policy, license/distribution, native platform qualification, and remaining interfaces are tracked in [the decision register](docs/decisions.md). Split focused commits and never push.
