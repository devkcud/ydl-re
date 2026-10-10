# Project status

Updated: October 9, 2026.

## Current state

Milestone A is complete. Milestone B now has verified persistence and HTTP resume. The CLI, inspection, and crash-process tests are implemented and passing in the working tree; their final documentation and commit are next.

## Handoff

- `internal/transfer/resume.go` validates source/effective fingerprints, exact range bounds, echoed strong ETags, encoding, and length before changing saved data. A 416 is re-probed and refused. Ready checkpoints recover locally without network access.

- `internal/storage/manifest.go` defines strict schema version 1. `completed = n` denotes `[0, n)`; URL identities and saved prefixes use SHA-256. Raw URLs and query tokens are not persisted.
- Review found and fixed JSON null values being decoded as zero/empty fields. Schema version 1 now requires every field to be present, unique, correctly typed, and non-null.
- `checkpoint.go` syncs bytes before advancing progress, checkpoints during transfer and shutdown, verifies saved hashes, and only truncates uncertain tails when the caller explicitly starts validated continuation.
- `file.go` preserves existing no-overwrite rules and reconciles `ready` partial/link/rename states. Complete checkpoints can finish publication locally after checksum validation.
- Persistent `.yoinkdl.lock` files provide a stable inode for nonblocking advisory locks. Kernel ownership ends when the process closes or dies; lock files are never unlinked automatically. New reserved artifact names are handled by filename derivation.
- The original design is unchanged. Go platform build directives keep Unix locking separate from unsupported targets; code otherwise has no explanatory comments and follows the spacing rule.

## Validation

- `go test ./internal/storage` passed with checkpoint, corruption, invalid-schema, ownership, and interrupted-publication cases.
- The null-field regression failed before the fix; `go test ./internal/storage` passes after explicit null rejection.
- `make check` passed after integration: formatting, vet, all tests, race checks, and build. The suite includes automatic/explicit resume, unchanged-artifact refusal cases, active inspection, and a real forced-process-kill/relaunch with final hash verification.
- Linux/amd64 is the tested runtime. Local test listeners require execution outside the sandbox. macOS native qualification, Windows storage, network filesystems, and real power-loss testing remain open.

## Next steps

1. Finish user-facing docs for automatic resume, `--resume`, `inspect`, persistent lock files, and current compatibility limits; commit the CLI slice separately.
2. Smoke-test the built executable against a local resumable server, audit documentation, and record the final Milestone B handoff.
3. Start Milestone C only after B is closed: plan disjoint ranges, preserve version identity, add bounded workers and safe retries/fallback with fault tests.

## Open decisions and blockers

No external blocker remains. Manifest/schema, lock recovery, URL privacy, checkpoint ordering, and refusal policies are documented in [the decision register](docs/decisions.md). License/distribution and later features remain open.

Update this handoff after every implementation step. Split focused local commits and never push.
