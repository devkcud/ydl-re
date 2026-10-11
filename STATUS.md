# Project status

Updated: October 11, 2026.

## Current state

Milestones A, B, and C are implemented and pass the local Linux gate. The CLI supports sequential/parallel transfers, safe resume, exclusive offset writes, verified checkpoints, bounded retries, inspection, cancellation, overwrite protection, and inactivity timeouts. Milestone D has started with the rate parser and shared budget; HTTP/CLI integration remains.

Bandwidth caps, JSON events, final terminal/security/platform qualification, configuration loading, and release tooling remain. Go 1.27 and the standard library are still the only runtime development requirements. A full v0.1 release is not complete.

## Handoff

- `internal/transfer/limit.go` owns strict positive integer byte-rate parsing and a shared cancellable budget. Decimal/binary units are case-sensitive, `/s` is required, fractions/zero/overflow are refused, and leading zeros are allowed. Library rate zero selects unlimited. Initial/idle credit is `min(65536, ceil(rate / 10))` bytes. Charge actual bounded reads, retain charges across failed/canceled attempts, and keep throttle waits outside network inactivity timers. This foundation is not yet wired into downloads.
- `internal/transfer/retry.go` owns retry classification and budgets. `--retries` accepts decimal 0..10 additional attempts (default three); zero disables repeats. `Request.Retries` uses nil for the default and an explicit zero pointer to disable retries. Configuration is validated before file/network work.
- Retry only 408/429/500/502/503/504 and selected transient network failures. Identity, bounds, encoding, malformed/oversized bodies, local storage, and cancellation errors are terminal. Equal-jitter backoff starts at 125–250 ms and caps at eight seconds. Valid Retry-After values may extend a wait to 30 seconds; longer hints stop instead of retrying early. Invalid/duplicate hints use ordinary backoff.
- Each probe, saved-range validation, or full-stream request setup has its own budget. A whole chunk shares one budget across request and body failures; there are no nested header budgets. Fresh fallback setup never repeats the preceding probe. Sequential body failures preserve checkpoints for later resume, and the diagnostic GET after 416 remains a single request followed by refusal.
- `workers.go` keeps each retry within its worker slot, buffer, immutable identity, and interval. Responses and writer claims close before backoff. Failed-attempt bytes are removed from useful progress before replacement, so progress can decrease without double-counting bytes or speed. Terminal/exhausted failures cancel peers; every worker joins before storage closes or publishes.
- `--connections`/`-c` remains 1..32 (default four), with one 64 KiB buffer per worker and at most 256 queued chunks. `internal/ranges` chooses fixed half-open chunks of at least four MiB, enlarged to keep at most 256 chunks. Resume keeps the saved chunk size and only fetches whole missing chunks.
- Manifest version 1 remains a checksummed sequential prefix. Version 2 stores disjoint whole chunks with individual hashes; saved-byte totals are never offsets. Original/effective URLs are stored only as hashes. No implicit schema conversion, destructive fallback, or legacy-partial import occurs.
- `range_resume.go` validates the saved strong ETag, effective URL, total, identity encoding, exact byte-zero bounds, and clean probe completion before activating writers. Empty saved ranges use If-Match with a known empty 200 response. Fully covered partials still validate remotely; only verified ready state recovers offline. Refused validation leaves existing artifacts unchanged.
- Storage holds a persistent advisory-lock inode, grants exclusive chunk writers, syncs bytes before atomic manifest publication, and rechecks exact coverage/hashes before final publication. Never unlink lock files for stale-owner recovery. Normal cancellation reaches requests, waits, and verification; OS sync itself remains non-interruptible. Completed destinations require explicit overwrite.
- Sequential and parallel process tests cover forced termination, lock release, inspection, and exact-hash recovery. The parallel fixture preserves two disjoint chunks plus wrong unconfirmed gap bytes in a full-sized partial, then resumes with a changed worker limit and requests only the missing chunks.
- The original design is unchanged. Keep focused local commits, update this handoff after meaningful steps, and never push.

## Validation

- `go test ./internal/transfer -run 'Test(ParseLimit|Limiter)' -count=1` passed. Controlled-clock tests cover aggregate budgets with 1/4/32 readers, idle burst bounds, cancellation without refunding received bytes, minimum/maximum rates, and strict unit parsing.
- Rate-foundation `make check` passed: formatting, vet, all tests, race checks, and build, including sequential/parallel forced-process recovery and actual CLI retry budgets.
- Documentation audit passed: 20 Markdown files, 180 local links/anchors, 23 shell blocks, two JSON examples, one TOML example, fences, and whitespace. The preceding Milestone C checks also verified manifest checksums, built-CLI inspection, and retry help. The original design is unchanged; its illustrative shell placeholders are excluded from runnable-example syntax checks.
- Environment: Go 1.27.2 on Linux/amd64. Local HTTP fixtures require listeners outside the sandbox. Native macOS, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Wire the shared budget into all HTTP body reads and expose `--limit`; cover sequential/fallback/resumed/parallel transfers, probe/retry accounting, cancellation, and timeout separation. Useful-byte progress must never refund or supply the limiter budget. Run the full gate and refresh docs.
2. Add JSON events and final terminal/error behavior, then complete security/platform/release qualification in roadmap order.

## Open decisions and blockers

No external blocker remains. Rate grammar/accounting is settled; JSON/event contracts, native platform qualification, license/distribution, and remaining interfaces are tracked in [the decision register](docs/decisions.md). The standalone CLI remains the priority; configuration files, daemons, queues, and TUIs are not prerequisites.
