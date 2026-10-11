# Project status

Updated: October 10, 2026.

## Current state

Milestones A and B are implemented and pass local Linux checks. The built CLI supports sequential HTTP(S) downloads, automatic compatible resume, explicit `--resume`, read-only `inspect`, progress, cancellation, overwrite protection, and network inactivity timeouts.

Milestone C is active. Range planning, version-2 checkpoints, exclusive bounded offset writers, fresh capability probing, and immutable representation validation are implemented. The CLI remains sequential: probe integration, HTTP workers, retries, bandwidth caps, JSON events, configuration loading, and release tooling remain. Go 1.27 and the standard library are still the only runtime development requirements.

## Handoff

- `internal/ranges` plans at most 256 fixed half-open chunks, defaulting to at least four MiB each. It validates sorted whole-chunk coverage, subtracts completed work, and converts HTTP endpoints without overflow. Empty plans contain no tasks. Large files use larger chunks and can lose more uncheckpointed work on interruption.
- `internal/storage/manifest.go` and `range_manifest.go` read distinct strict schemas. Version 1 keeps `completed = n` as prefix `[0, n)` with its digest. Version 2 stores `chunk_size` and `completed_ranges` with per-chunk digests; saved-byte totals are derived, never resume offsets. No implicit schema conversion occurs. URL identities remain hashed.
- `range_writer.go` adds `ClaimRange` and bounded `RangeWriter` writes. Active/completed chunks cannot be claimed again; stale writers cannot write after release. `Commit` serializes sync/hash/checkpoint publication and releases the claim only after success. Call it only after validating clean HTTP completion; `Close` releases failed attempts without acknowledging them. File close/publication refuses active writers.
- `range_checkpoint.go` retains explicit v2 initialization/recovery and verified checkpoints. `FinalizeContext` checks cancellation during range hashes and before publication; OS sync remains non-interruptible. Existing version-1 APIs and ready recovery stay compatible.
- Version-1 checkpoint timing and resume behavior remain unchanged: sync before saving after four MiB/one second and on normal interruption; truncate unconfirmed tails only after a compatible response. Range checkpoints acknowledge whole chunks only, and retain initialized state even with zero saved bytes.
- Stable `.yoinkdl.lock` inodes hold nonblocking advisory locks. Process death releases ownership, so do not unlink lock files for stale-owner recovery. Orphan temporary manifests are ignored.
- `internal/transfer/resume.go` validates echoed strong ETags, identity encoding, source/effective URL fingerprints, exact range bounds, and lengths before reuse. Changed/unsupported responses preserve existing artifacts. A 416 is re-probed and refused.
- `internal/transfer/representation.go` captures eligible identity by value and builds conditional requests without retaining mutable response data. Its exact range-bound validator is shared with version-1 resume. Workers must still check full body completion before acknowledging a chunk.
- `internal/transfer/probe.go` proves fresh range capability with `GET bytes=0-0`, strict headers, and exactly one byte with clean EOF. It returns a selected identity or an open full-response stream. The coordinator must consume/close that stream and use its own metadata. Ignored ranges reuse the untouched 200 body; ineligible valid 206/416 responses get one fresh full GET after discarding probe bytes. Invalid probes fail. No storage writes or saved-state fallback occur.
- Probe and fallback bodies enforce per-read inactivity timeouts and cancellation. The probe is not wired into `Download` yet; keep its fresh-only fallback separate from existing version-1 resume and version-2 partial refusal.
- Both schemas support verified offline `ready` recovery, including interrupted link/rename publication. `inspect` reads either schema without hashing or acquiring writer ownership. The current downloader creates version 1 and refuses version-2 partials before network access; it never sends an aggregate saved-byte count as a suffix offset.
- The original design is unchanged. The spacing rule, handoffs after every implementation, focused local commits, and no-push rule remain in force.

## Validation

- `go test -race ./internal/storage` and `make check` passed for exclusive range writers, concurrent checkpoint serialization, and context-aware finalization. Documentation checks passed for 20 Markdown files, 175 local links/anchors, fences, and whitespace; examples are unchanged.
- `go test ./internal/transfer -run 'TestRepresentation|TestResume' -count=1` and `go test ./internal/transfer -run '^TestProbe' -count=1` passed outside the sandbox after localhost listeners were denied inside it. Tests cover immutable identity, strict headers/body completion, fresh fallback with changed versions, 416, exact request bounds, timeout/cancellation, and response cleanup.
- `make check` passed for both the representation helpers and the final probe implementation: formatting, vet, all tests, race checks, and build. Sandbox restrictions on listeners/Go cache require the approved outside-sandbox environment.
- Existing sequential HTTP and process-death tests pass, including actual killed-writer suffix recovery. New tests prove partial range refusal before network access and offline ready publication. Range fixtures use direct deterministic offset writes; concurrent workers and parallel HTTP remain unqualified.
- Documentation audit passed: 20 Markdown files, 174 local links/anchors, 23 shell blocks, two JSON examples, one TOML example, fences, and whitespace. Both manifest examples passed checksum checks and built-CLI inspection. The original design remains unchanged; its illustrative shell placeholders are excluded from runnable-example syntax checks.
- Environment: Go 1.27.2 on Linux/amd64. HTTP tests need localhost listeners outside the sandbox. macOS native checks, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Implement bounded HTTP workers using `ClaimRange`, resolving worker/queue/buffer limits. Wire `probe` into fresh downloads after acquiring output ownership and consume or close its returned sequential stream. Keep version-1 resume unchanged.
2. Serialize range checkpoints, re-fetch whole missing chunks, propagate cancellation through verification, and wire version-2 resume only after validating the stored identity remotely. Keep destination protection and refusal guarantees.
3. Add bounded retry/backoff and define the connection/retry flag contracts. Test ignored ranges, changing versions, failed chunk replacement, cancellation, and concurrent completion.
4. Complete focused/full gates and process-recovery evidence for parallel transfers, update this handoff, and close C before hardening in D.

## Open decisions and blockers

No external blocker remains. Partial reuse requires a known length, identity encoding, a matching strong ETag, and exact URL fingerprints. Refreshed signed URLs, weak/unversioned/encoded partials, and old partials without manifests are refused rather than silently restarted.

A full v0.1 release is not complete. Worker/queue/buffer limits, retry policy, license/distribution, native platform qualification, and remaining interfaces are tracked in [the decision register](docs/decisions.md). Split focused commits and never push.
