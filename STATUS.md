# Project status

Updated: October 9, 2026.

## Current state

Milestone A is complete. Milestone B's storage APIs now implement manifests, verified prefix checkpoints, stable OS locking, and interrupted-publication recovery. HTTP resume and the CLI commands are not connected yet; the executable still exposes the Milestone A interface.

## Handoff

- `internal/storage/manifest.go` defines strict schema version 1. `completed = n` denotes `[0, n)`; URL identities and saved prefixes use SHA-256. Raw URLs and query tokens are not persisted.
- `checkpoint.go` syncs bytes before advancing progress, checkpoints during transfer and shutdown, verifies saved hashes, and only truncates uncertain tails when the caller explicitly starts validated continuation.
- `file.go` preserves existing no-overwrite rules and reconciles `ready` partial/link/rename states. Complete checkpoints can finish publication locally after checksum validation.
- Persistent `.yoinkdl.lock` files provide a stable inode for nonblocking advisory locks. Kernel ownership ends when the process closes or dies; lock files are never unlinked automatically. New reserved artifact names are handled by filename derivation.
- The original design is unchanged. Go platform build directives keep Unix locking separate from unsupported targets; code otherwise has no explanatory comments and follows the spacing rule.

## Validation

- `go test ./internal/storage` passed with checkpoint, corruption, invalid-schema, ownership, and interrupted-publication cases.
- `make check` passed: formatting, vet, all tests, race checks, and build against the existing CLI.
- Linux/amd64 is the tested runtime. Local test listeners require execution outside the sandbox. macOS native qualification, Windows storage, network filesystems, and real power-loss testing remain open.

## Next steps

1. Connect HTTP metadata to manifests and implement conservative GET-based resume validation: source/effective fingerprints, strong ETag, total size, encoding, and exact response bounds.
2. Add automatic compatible resume, explicit `--resume`, and read-only `inspect`; reject mismatches without changing saved artifacts. Keep resumed progress/rate accounting accurate.
3. Add end-to-end interruption/relaunch and forced-process-termination tests, refusal cases, and binary smoke checks. Run the full gate and refresh user docs.
4. Complete Milestone B before parallel ranges/retries in C and release hardening in D.

## Open decisions and blockers

No external blocker remains. Manifest/schema, lock recovery, URL privacy, checkpoint ordering, and refusal policies are documented in [the decision register](docs/decisions.md). License/distribution and later features remain open.

Update this handoff after every implementation step. Split focused local commits and never push.
