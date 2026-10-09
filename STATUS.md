# Project status

Updated: October 9, 2026.

## Current state

Milestone A is in progress. The Go 1.27 module (`yoinkdl`), safe storage layer, and sequential HTTP engine are implemented and tested. The CLI and requested Makefile are being wired and are not yet validated.

## Handoff

`internal/transfer` streams full HTTP(S) responses with a 64 KiB buffer, exact known-length checks, unknown-length completion, cancellation, and a network inactivity timeout. It preserves raw representation bytes, bounds redirects, rejects HTTPS downgrade/URL credentials, and avoids URL tokens in request errors.

`AGENTS.md` requires readable spacing and blank lines between logical blocks. Apply this to new code and existing code being changed.

`internal/storage` exclusively creates private partial files, rejects incompatible artifacts and unsafe output types, preserves nonempty failed transfers, and syncs before publication. Default hard-link publication cannot replace a destination that appears mid-transfer; explicit overwrite uses rename. Directory-sync and cleanup failures are reported separately from publication.

The filesystem must support hard links and directory sync. Downloads are enabled on Linux/macOS; only Linux is locally tested. Windows publication is deferred. The original engineering design is unchanged.

## Validation

- `go test -race ./internal/storage` and `go vet ./internal/storage` passed.
- `go test ./internal/transfer`, `go test -race ./internal/transfer`, and `go vet ./internal/transfer` passed. The final race run also covers slow active transfers and repeated redirect credential handling.
- The sandboxed Go invocation could not resolve installed standard packages; the same toolchain works outside the sandbox.

## Next steps

1. Finish and test `download`, `version`, help, signal handling, terminal progress, and the Makefile build/test/check targets.
2. Exercise the binary against local servers; run formatting, build, tests, vet, and race checks, then update the user docs to reflect actual commands.
3. Complete Milestone A evidence before starting Milestone B manifests, validator-based resume, and `inspect`.

## Open decisions and blockers

No external blocker remains for Milestone A. Go checks currently require execution outside the sandbox. License, distribution, Windows publication, native macOS qualification, and the later manifest/CLI contracts remain open in [the decision register](docs/decisions.md).

Update this handoff after every implementation step. Split focused local commits and never push.
