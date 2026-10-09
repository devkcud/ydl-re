# Project status

Updated: October 9, 2026.

## Current state

Milestone A is in progress. The Go 1.27 module (`yoinkdl`) and safe storage layer are implemented. The HTTP engine, executable, and requested Makefile are next. The existing design/documentation baseline is committed locally.

## Handoff

The HTTP engine is being implemented and is not validated yet. `AGENTS.md` now requires readable spacing and blank lines between logical blocks; apply this to new code and existing code being changed.

`internal/storage` exclusively creates private partial files, rejects incompatible artifacts and unsafe output types, preserves nonempty failed transfers, and syncs before publication. Default hard-link publication cannot replace a destination that appears mid-transfer; explicit overwrite uses rename. Directory-sync and cleanup failures are reported separately from publication.

The filesystem must support hard links and directory sync. Downloads are enabled on Linux/macOS; only Linux is locally tested. Windows publication is deferred. The original engineering design is unchanged.

## Validation

- `go test ./internal/storage` passed before the final ownership cases were added.
- `go test -race ./internal/storage` passed with concurrent ownership, collisions, symlinks, preservation, and sync-failure cases.
- `go vet ./internal/storage` passed.
- The sandboxed Go invocation could not resolve installed standard packages; the same toolchain works outside the sandbox.

## Next steps

1. Implement the sequential HTTP engine with input validation, bounded buffers, timeouts, cancellation, byte-count verification, and safe progress events.
2. Add `download`, `version`, help, signal handling, terminal progress, and the Makefile build/test/check targets.
3. Exercise the binary against local servers; run formatting, build, tests, vet, and race checks, then update the user docs to reflect actual commands.
4. Complete Milestone A evidence before starting Milestone B manifests, validator-based resume, and `inspect`.

## Open decisions and blockers

No external blocker remains for Milestone A. Go checks currently require execution outside the sandbox. License, distribution, Windows publication, native macOS qualification, and the later manifest/CLI contracts remain open in [the decision register](docs/decisions.md).

Update this handoff after every implementation step. Split focused local commits and never push.
