# Changelog

## Unreleased

- Internal v0.2 queue foundation: shared request validation, strict bounded snapshots, and a private Linux atomic store with exclusive ownership. Sequential/concurrent queue settings persist; scheduling and daemon commands remain under development.

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
