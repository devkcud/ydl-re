# Project status

Updated: October 9, 2026.

## Current state

Milestone A is implemented and passes local Linux checks. `bin/yoinkdl` is built and supports `download`, `version`, help, progress, cancellation, explicit overwrite, and network inactivity timeouts. The Go 1.27 module uses only the standard library.

The requested Makefile provides build, test, vet, race, formatting, and full-check targets. Resume, manifests, `inspect`, parallel ranges, retries, bandwidth caps, JSON output, and configuration loading remain unimplemented.

## Handoff

- `cmd/yoinkdl` wires SIGINT/SIGTERM to cancellation. `internal/cli` parses options before or after the URL, prints progress/errors to stderr and the absolute completed path to stdout, and returns `130` for interruption.
- `internal/transfer` streams full HTTP(S) responses with a 64 KiB buffer, known-length verification, clean unknown-length completion, and a default 30-second inactivity timeout. It preserves representation bytes, bounds redirects, and avoids URL tokens in request errors.
- `internal/storage` exclusively creates private partial files, rejects incompatible artifacts, preserves nonempty failures, and syncs before publication. Default hard-link publication cannot replace an unexpected destination; explicit overwrite uses rename. Errors retain whether publication already happened.
- `AGENTS.md` requires readable spacing, a current handoff after every implementation, focused local commits, and no pushes. The original design remains unchanged.

The filesystem must support hard links and directory sync. Linux is tested locally; macOS still needs native qualification. Windows downloads are explicitly unsupported. Preserved partials cannot yet be resumed.

## Validation

- `make check` passed: formatting, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the executable build.
- Tests cover exact fixture hashes, encoded and unknown-length bodies, empty files, bad statuses, short bodies, cancellation, inactivity versus active slow transfers, redirects, secret-safe errors, collisions, concurrent ownership, unsafe paths, sync failure, CLI parsing/output, and real process interruption.
- `./bin/yoinkdl version` and `./bin/yoinkdl download --help` passed against the built executable.
- Built-executable smoke checks passed on the workspace filesystem: known and unknown lengths matched fixture hashes; a truncated response failed and preserved only partial data.
- Runtime environment: Go 1.27.2 on Linux/amd64. Tests needed execution outside the sandbox to permit localhost listeners; all runtime gates passed there.
- Documentation audit passed: 19 pages, 173 local links/anchors, 19 shell examples, JSON/TOML examples, and whitespace. The original design checksum is unchanged.

## Next steps

1. Start Milestone B by defining the manifest schema, persisted interval convention, and recovery behavior after interrupted publication. Preserve the current no-overwrite and exclusive-ownership rules.
2. Implement metadata probing and strong representation identity, including explicit handling of missing/weak validators, changed content, and refreshed URLs.
3. Add conservative checkpoints and compatible sequential resume with safe lock recovery. Never reinterpret existing untracked partial bytes as verified progress.
4. Add `inspect`, `--resume`, and deterministic interrupt/relaunch and crash-recovery tests; update the CLI/docs with adopted behavior.
5. Complete Milestone B evidence before moving to parallel ranges/retries in C and release hardening in D.

## Open decisions and blockers

No external blocker remains for the next implementation step. License/distribution, native macOS qualification, Windows publication, manifest/recovery contracts, and later interfaces remain open in [the decision register](docs/decisions.md).

This is the first working milestone, not the complete v0.1 release. Update this handoff after every implementation step, split focused commits, and never push.
