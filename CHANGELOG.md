# Changelog

## Unreleased

## [0.2.0-rc.1]

First Linux/amd64 candidate for the optional daemon and persistent queue. Standalone downloads remain independent.

- Explicit foreground `daemon start/stop/set` and `add/list/pause/resume/cancel/forget` commands, with no automatic daemon startup.
- Durable FIFO admission, up to 256 retained jobs, and live whole-download concurrency from 1 through 8, separate from each download's worker count.
- Private Linux state and Unix-socket control with exclusive ownership, peer-UID checks, strict bounded messages, and joined shutdown.
- A daemon-wide bandwidth cap composed with per-download caps, retained across job transitions and live concurrency changes.
- Safe text/JSON results, caller-relative output paths, and preserved job IDs after lost acknowledgments. Failed client output leaves accepted work running.
- Explicit resume after interrupted attempts, conservative publication outcomes after crashes, and metadata-only forget that preserves final files and checkpoints. Queued transfers never overwrite existing final files.
- Local crash, capacity, concurrency, and shared-rate process qualification; native ordinary/race/CGO-disabled gates and six-target compilation pass.

Queue files intentionally retain full source URLs, including query tokens, as private plaintext until completion/cancellation or explicit forget. Transfer manifests remain fingerprint-only. Linux/amd64 is the only qualified native target; other builds provide compilation evidence, and non-Linux daemon operations remain unsupported. Hosted candidate verification and publication are separate from local preparation.

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
