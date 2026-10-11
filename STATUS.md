# Project status

Updated: October 11, 2026.

## Current state

Milestones A, B, and C are implemented and pass the local Linux gate. The CLI supports sequential/parallel transfers, safe resume, exclusive offset writes, verified checkpoints, bounded retries, inspection, cancellation, overwrite protection, and inactivity timeouts. Milestone D includes aggregate body-rate caps through `--limit` and versioned newline-delimited output through `--json`.

Final terminal/security/platform qualification, configuration loading, and release tooling remain. Go 1.27 and the standard library are still the only runtime development requirements. A full v0.1 release is not complete.

## Handoff

- `download --json` uses `internal/cli/events.go` for schema-version-1 progress/result records. Unknown totals/paths use null, quiet mode suppresses only progress, and counters remain advisory. Results distinguish completed/failed/interrupted status from publication/preserved partials; `published: true` can accompany a cleanup error. Null paths do not prove existing artifacts are absent. Error text is diagnostic; raw URLs/headers/validators are absent. See [the contract](docs/cli.md#json-output).
- `cli/json.go` cancels the transfer on output failure, waits for `Download` cleanup, then emits the terminal result. Normal JSON failures/interruption go only to stdout; parsing errors/help remain text. Unusable output reports known publication/partial paths on stderr and exits 1 (130 for caller interruption). `--json=false` retains text behavior. New output setup failures occur before transfer work.
- `output.go` detects short writes and gives deadline-capable writes five seconds, with cancellation during progress writes. `output_unix.go` uses `F_DUPFD_CLOEXEC` with minimum descriptor 3 so even closed stderr cannot cause fatal SIGPIPE before cleanup; normal close restores original flags and caller ownership. Synchronous backpressure has no event queue. Interrupted writes with zero bytes can still emit a terminal result; partial records poison the stream. Generic non-deadline writers retain their own blocking behavior.
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
- `go test -race ./internal/cli ./cmd/yoinkdl -run 'Test(JSON|RecordWriter|ProcessJSON)' -count=1` passed, including actual JSON SIGINT/resume and broken/full stdout pipes. `go test -race ./internal/cli -run 'TestJSON(RangeResume|EarlyFailures)' -count=1` passed for disjoint saved chunks/retries and unusable-output refusal.
- The new closed-stderr process regression initially failed with fatal SIGPIPE. Restricting the duplicate to descriptors 3 and above fixed it; the focused race command above includes the passing regression.
- Final integrated `make check` passed: formatting, vet, all tests, race checks, and build. Existing recovery, retry, and rate suites still pass alongside JSON process/pipe checks.
- Documentation audit passed: 20 Markdown files, 201 local links/anchors, 26 shell blocks, four JSON examples, one TOML example, fences, and whitespace. Built-CLI help and documented result-field checks passed, including quiet failure framing, source redaction, and refusal without artifacts. The original design is unchanged; illustrative placeholders are excluded from runnable-example syntax checks.
- Environment: Go 1.27.2 on Linux/amd64. The full HTTP gate runs outside the sandbox to permit localhost listeners. Native macOS, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Qualify final text terminal/error behavior: redirected output and help/diagnostic write failures, path/control-character rendering, interrupt handling, and accurate state-preservation messages. Reuse the established JSON result semantics and output ownership where appropriate.
2. Complete security review, platform/build/native filesystem qualification, and licensing/distribution/release tooling in roadmap order. Configuration files and daemon/TUI work remain optional/later scope.

## Open decisions and blockers

No external blocker remains. Rate accounting and CLI JSON version 1 are settled. Future subscriber/retry counters, native platform qualification, license/distribution, and remaining interfaces are tracked in [the decision register](docs/decisions.md). The standalone CLI remains the priority; configuration files, daemons, queues, and TUIs are not prerequisites.
