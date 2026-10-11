# Project status

Updated: October 11, 2026.

## Current state

Milestones A, B, and C are implemented and pass the local Linux gate. The CLI supports sequential/parallel transfers, safe resume, exclusive offset writes, verified checkpoints, bounded retries, inspection, cancellation, overwrite protection, and inactivity timeouts. Milestone D includes aggregate body-rate caps through `--limit`; the JSON encoder/output foundation is implemented, with flag integration next.

JSON events, final terminal/security/platform qualification, configuration loading, and release tooling remain. Go 1.27 and the standard library are still the only runtime development requirements. A full v0.1 release is not complete.

## Handoff

- `internal/cli/events.go` defines schema-version-1 newline-delimited progress/result records. Unknown totals/paths use null, quiet mode suppresses only progress, counters remain advisory, and results separate status/exit code from publication/preserved partials. Error text is diagnostic; raw URLs/headers/validators are absent. `--json` is not exposed yet; [the contract](docs/cli.md#json-output) is settled.
- `output.go` detects short writes, bounds deadline-capable writes to five seconds, and cancels pending progress writes through deadlines. `output_unix.go` duplicates Linux/macOS pipe/socket handles, preserves caller ownership/flags, and avoids fatal stdout SIGPIPE before transfer cleanup. Synchronous backpressure has no event queue. Cancellation before any bytes can still emit a terminal record; partial records poison the stream. Generic non-deadline writers retain their own I/O blocking behavior.
- `internal/transfer/limit.go` owns `--limit` parsing and one cancellable budget per download. Positive integer decimal/binary byte units require `/s`, are case-sensitive, and allow leading zeros; fractions/zero/overflow are refused. Omission is unlimited; `Request.Limit` uses bytes/second with zero for unlimited and rejects negatives before file/network work. Initial/idle credit is `min(65536, ceil(rate / 10))` bytes.
- `body.go` wraps all HTTP response bodies, including probes, redirect drains, fallback streams, and retries. Bound each read by the burst, charge actual bytes before delivery, and retain charges after failed/canceled attempts. Saved bytes/progress rollback never supply credit. Network inactivity timers wrap only actual reads; throttle waits are cancellable and excluded. Each response has its own cancellation context; closing it cancels that request. Socket/HTTP read-ahead and protocol overhead are outside the application body cap. Ready recovery remains offline.
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

- `go test -race ./internal/cli -run 'Test(JSON|RecordWriter)' -count=1` passed: exact 64-bit counters, progress coalescing/rollback, null totals, quiet mode, terminal outcomes, escaping, short writes, deadlines, cancellation, and descriptor ownership/flag restoration.
- JSON-foundation `make check` passed: formatting, vet, all tests, race checks, and build. The existing recovery/retry/rate suites still pass. Rate timing tolerances remain one microsecond under controlled time and five milliseconds for HTTP sampling.
- Documentation audit passed: 20 Markdown files, 189 local links/anchors, 24 shell blocks, four JSON examples, one TOML example, fences, and whitespace. The original design is unchanged; illustrative placeholders are excluded from runnable-example syntax checks.
- Environment: Go 1.27.2 on Linux/amd64. The full HTTP gate runs outside the sandbox to permit localhost listeners. Native macOS, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Wire `download --json` to the event/output foundation. Cancel the transfer on output failure, join/close before terminal reporting, preserve text-mode behavior, and cover real CLI success/resume/failure/interruption and broken/full output pipes. Run the full gate and refresh docs.
2. Complete final terminal/error behavior, then security/platform/release qualification in roadmap order.

## Open decisions and blockers

No external blocker remains. Rate grammar/accounting is settled; JSON/event contracts, native platform qualification, license/distribution, and remaining interfaces are tracked in [the decision register](docs/decisions.md). The standalone CLI remains the priority; configuration files, daemons, queues, and TUIs are not prerequisites.
