# Changelog

## Unreleased

- Internal v0.2 queue foundation: shared request validation, strict bounded snapshots, and a private Linux atomic store with exclusive ownership. Full source URLs and sequential/concurrent queue settings persist privately.
- Queue coordinator with FIFO admission, live concurrency limits, pause/resume/cancel/forget, conservative recovery, safe polling views, and draining shutdown. Private Linux IPC and composed shared/per-transfer bandwidth are implemented.

- Linux v0.2 development CLI: explicit foreground daemon start/stop/set, add/list/pause/resume/cancel/forget, safe text/JSON results, caller-relative outputs, and preserved job IDs after lost acknowledgments. Broken client output does not cancel durable jobs. Final v0.2 release qualification remains pending.
- Local Linux crash qualification covers saved queue order/settings, explicit checkpoint resume, pending pause/cancel intents, and preserved final files after death between publication and queue completion.
- Retained queue capacity and forget recovery now have real daemon process evidence: full terminal queues survive restart, forget durably releases capacity, final files remain protected, and preserved partials can be readmitted safely.
- Process qualification covers live concurrency changes, retained cancellation slots, composed private/shared bandwidth, shared credit across controls and job transitions, and persisted private caps across restart.

## [0.1.0]

Initial stable release for Linux/amd64, with the same downloader behavior as `0.1.0-rc.1`.

- Foreground HTTP(S) downloads with sequential and bounded parallel transfers.
- Checksummed prefix/chunk resume, stable file ownership, and verified publication.
- Bounded retries, inactivity timeouts, aggregate rate limits, and cancellation.
- Read-only checkpoint inspection, terminal progress, and versioned JSON output.
- Credential-safe diagnostics, protected partial files, and sanitized automatic filenames.
- Linux/amd64 binary packaging with matching source archives and SHA-256 checksums.
- MPL 2.0 licensing and public GitHub Issues security reporting.
- Additional Linux qualification for trusted HTTPS transfers and untrusted-certificate refusal.

Linux/arm64 and macOS have compilation evidence only; Windows download/inspection storage is unsupported.

## [0.1.0-rc.1]

- Foreground HTTP(S) downloads with sequential and bounded parallel transfers.
- Checksummed prefix/chunk resume, stable file ownership, and verified publication.
- Bounded retries, inactivity timeouts, aggregate rate limits, and cancellation.
- Read-only checkpoint inspection, terminal progress, and versioned JSON output.
- Credential-safe diagnostics, protected partial files, and sanitized automatic filenames.
- Initial Linux/amd64 binary packaging with matching source archives and SHA-256 checksums.
- MPL 2.0 licensing and public GitHub Issues security reporting.

The initial binary target is Linux/amd64. Linux/arm64 and macOS have compilation evidence only; Windows download/inspection storage is unsupported.
