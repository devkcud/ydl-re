# Project status

Updated: October 11, 2026.

## Current state

Milestones A, B, and C are implemented and pass the local Linux gate. The CLI supports sequential/parallel transfers, safe resume, exclusive offset writes, verified checkpoints, bounded retries, inspection, cancellation, overwrite protection, and inactivity timeouts. Milestone D includes aggregate body-rate caps through `--limit` and versioned newline-delimited output through `--json`.

Text terminal/error handling is implemented with local Linux evidence. The security review is active; platform qualification, configuration loading, and release tooling remain. Go 1.27 and the standard library are still the only runtime development requirements. A full v0.1 release is not complete.

## Handoff

- The security review is in progress. `cli.go` now applies recognized options through `flag.Set` and reports only a known option name when validation fails. Invalid values, malformed option names, and parser error text cannot echo credentials. Existing boolean/inline/separated syntax, options around the URL, help, and `--` remain supported.
- HTTP body errors now use the existing safe classification, including malformed trailers. The private transport disables connection reuse to prevent Go's idle HTTP/1 diagnostic from logging unsolicited bytes; subsequent requests pay connection setup costs. Direct/fallback/zero-prefix full responses share `200`/no-Content-Range validation before saved tails can be truncated. See [HTTP handling](docs/http.md).
- Mutable partials now require exactly one hard link and private Unix permission bits. Recovery and writes reject aliases/shared access without altering bytes, modes, or metadata. Verified `ready` recovery still accepts final/partial links from interrupted publication. Sequential finalization reads the whole file back and checks size/checksum against the received stream; failed verification disables further checkpointing so close preserves the prior manifest. See [publication](docs/resume-and-storage.md#final-publication-and-recovery).
- Text/JSON output shares bounded writes, short-write detection, and cancellable owned descriptors for Linux/macOS pipes, sockets, and detected terminals. Output failures stop transfers, join workers, and preserve verified published files. Invalid invocations retain exit 2, other failures use 1, and caller interruption uses 130. Text escapes nonprinting characters; ANSI progress requires actual geometry and nonempty, non-`dumb` TERM. See [CLI output](docs/cli.md#json-output).
- `--limit` uses one cancellable body-byte budget across probes, redirects, workers, and retries. Received bytes stay charged after failure; saved progress supplies no credit. Throttle waits are excluded from inactivity timeouts. Protocol overhead and socket read-ahead are outside the cap. See [accounting](docs/concurrency.md#aggregate-rate-cap).
- `--connections`/`-c` accepts 1..32 (default four); `--retries` accepts 0..10 additional attempts (default three). Each whole chunk shares one request/body budget. Identity/protocol/storage failures are terminal; selected transient failures use bounded jitter and Retry-After. Sequential body failures require later resume. Workers release failed claims before waiting and roll back failed progress before replacement.
- Version 1 remains a checksummed sequential prefix; version 2 stores disjoint whole chunks and keeps the saved chunk size on resume. Only hashes of original/effective URLs are stored. Local checks and conditional GET validation precede writes; refused validation preserves artifacts. Only a verified `ready` checkpoint recovers offline. See [storage](docs/resume-and-storage.md) and [HTTP](docs/http.md).
- Storage holds a persistent advisory-lock inode, grants exclusive chunk writers, syncs bytes before atomic checkpoints, and validates completion before publication. Never unlink lock files to infer stale ownership. Completed destinations require explicit overwrite. Normal cancellation reaches requests, waits, and verification; OS sync itself remains non-interruptible.
- Forced-process-termination tests cover sequential/parallel recovery, stable locks, unconfirmed tail/gap bytes, changed worker counts, and exact fixture hashes. The original design remains unchanged. Keep focused local commits and never push.

## Validation

- Security regressions reproduced credential-bearing invalid option values, raw malformed HTTP trailer errors, unsolicited HTTP transport logs, and acceptance of contradictory full-response headers. These regressions now pass, including preserved partials after malformed trailers and unchanged zero-prefix artifacts after refused headers.
- `go test ./internal/cli ./internal/transfer ./cmd/yoinkdl -run 'Test(InvalidOptionsDoNotEcho|SequentialTrailerErrors|SingleStreamRejects|ProcessDoesNotLog|DownloadOptions|InvalidArguments|Resume)' -count=1` passed after the initial fixes.
- `make check` passed after the initial security changes: formatting, vet, all tests, race tests, and build, including the new zero-prefix full-response preservation test.
- Storage regressions initially reproduced writes through hard-linked/shared-permission partials and publication of changed, truncated, or extended sequential files. `go test -race ./internal/storage -count=1` and the subsequent `make check` pass, including late-created aliases, both schemas, unchanged refused artifacts, and legitimate interrupted link/rename recovery.
- Previous text/JSON/rate/recovery/retry gates and the documentation audit passed. The audit covered 20 Markdown files, 205 local links/anchors, 26 shell blocks, four JSON examples, and one TOML example; illustrative placeholders and original-design hard breaks are excluded from runnable/whitespace checks.
- Environment: Go 1.27.2 on Linux/amd64. HTTP/race tests run outside the sandbox to permit localhost listeners. Native macOS, Windows storage, network filesystems, and real power-loss durability remain unqualified.

## Next steps

1. Finish the v0.1 security review: check derived filename presentation, then record reviewed URL/header/manifest/path boundaries, regression evidence, and limits in the security/decision guides.
2. Complete platform/build/native filesystem qualification, licensing/distribution, and release tooling in roadmap order. Configuration files and daemon/TUI work remain optional/later scope.

## Open decisions and blockers

No external blocker remains. Rate accounting and CLI JSON version 1 are settled. Future subscriber/retry counters, native platform qualification, license/distribution, and remaining interfaces are tracked in [the decision register](docs/decisions.md). The standalone CLI remains the priority; configuration files, daemons, queues, and TUIs are not prerequisites.
