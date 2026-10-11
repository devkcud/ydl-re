# Project status

Updated: October 10, 2026.

## Current state

Milestones A and B are implemented and pass local Linux checks. The built CLI supports sequential HTTP(S) downloads, automatic compatible resume, explicit `--resume`, read-only `inspect`, progress, cancellation, overwrite protection, and network inactivity timeouts.

Milestone C is active. Its bounded range planner is implemented; disjoint persistence, parallel HTTP transfers, retries, bandwidth caps, JSON events, configuration loading, and release tooling remain unimplemented. Go 1.27 and the standard library are still the only runtime development requirements; the Makefile provides the full check gate.

## Handoff

- `internal/ranges` plans at most 256 fixed half-open chunks, using at least four MiB per chunk. It validates sorted whole-chunk coverage, subtracts completed work, and converts HTTP endpoints without overflow. Zero-length objects need no requests. Range persistence must retain the chosen chunk size; version 1 remains unchanged.
- `internal/storage/manifest.go` defines strict, non-null schema version 1. `completed = n` denotes the durable prefix `[0, n)` and `sha256` verifies that prefix. Original/effective URL identities are hashed; raw URLs/query tokens are not persisted.
- `checkpoint.go` syncs bytes before publishing snapshots after four MiB or one second of arriving data and on normal interruption. Recovery verifies saved bytes; unconfirmed tails are truncated only after a compatible remote response.
- Stable `.yoinkdl.lock` inodes hold nonblocking advisory locks. Process death releases ownership, so do not unlink lock files for stale-owner recovery. Orphan temporary manifests are ignored.
- `internal/transfer/resume.go` validates echoed strong ETags, identity encoding, source/effective URL fingerprints, exact range bounds, and lengths before reuse. Changed/unsupported responses preserve existing artifacts. A 416 is re-probed and refused.
- Verified `ready` checkpoints recover interrupted publication locally after complete checksum validation, including link/rename cases. This can finish an already received resource version without contacting the server.
- `internal/cli` exposes resume and inspect. Inspection reads metadata only; percentage includes saved bytes, while speed excludes them. `cmd/yoinkdl/resume_test.go` exercises actual process death, lock release, inspection, and suffix recovery to the expected hash.
- The original design is unchanged. The spacing rule, handoffs after every implementation, focused local commits, and no-push rule remain in force.

## Validation

- `go test ./internal/ranges` passed: uneven/small/empty plans, all completion subsets of small fixtures, invalid coverage, task bounds, and maximum signed 64-bit offsets.
- `make check` passed for the planner: formatting, vet, all tests, race checks, and build. The sandbox attempt could not write the Go cache; the approved outside-sandbox run passed.
- Resume tests cover automatic/explicit/chunked/zero-prefix requests, corrupt or missing state, unchanged-artifact refusal, changed identities, invalid bounds, missing/weak validators, 416 re-probing, interrupted bodies, and offline ready recovery.
- Storage tests cover durable prefix ordering, unconfirmed tails, conflicting writers, strict JSON/schema validation, and interrupted ready/link/rename publication. The explicit-null regression failed before the fix and passes afterward.
- The process test kills a writer after a durable checkpoint, starts a new process with `--resume`, and verifies the complete fixture hash.
- Built-executable smoke test passed: SIGINT returned `130`, inspection reported the checkpoint, resume fetched the suffix after 131072 saved bytes, and the complete file hash matched. Raw URL tokens were absent from metadata.
- Documentation audit passed: 20 Markdown files, 166 local links/anchors, fences, and whitespace. The preceding Milestone B audit also validated shell/JSON/TOML examples and the manifest checksum; those examples and the original design are unchanged in this step.
- Environment: Go 1.27.2 on Linux/amd64. HTTP tests need localhost listeners outside the sandbox. macOS native checks, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Add versioned persistence for disjoint completed chunks using the implemented planner. Preserve version-1 checkpoint compatibility explicitly; do not reinterpret the sequential prefix format as a range array.
2. Add capability probing and immutable representation identity for fresh parallel transfers, using the existing strict response validators and safe sequential fallback where applicable.
3. Implement bounded workers with exclusive interval ownership, offset writes, exact coverage verification, and conservative cancellation/checkpointing. Keep current resume refusal and destination protection guarantees.
4. Add bounded retry/backoff and define the connection/retry flag contracts. Test ignored ranges, changing versions, overlaps/gaps, failed interval replacement, cancellation, and concurrent completion.
5. Complete the focused and full gates, update this handoff, and close C before bandwidth/JSON/release hardening in D.

## Open decisions and blockers

No external blocker remains. Partial reuse requires a known length, identity encoding, a matching strong ETag, and exact URL fingerprints. Refreshed signed URLs, weak/unversioned/encoded partials, and old partials without manifests are refused rather than silently restarted.

A full v0.1 release is not complete. License/distribution, native platform qualification, future range-state evolution, and remaining interfaces are tracked in [the decision register](docs/decisions.md). Split focused commits and never push.
