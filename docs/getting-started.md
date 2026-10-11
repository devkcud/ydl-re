# Getting started

[Documentation index](README.md)

The CLI supports fresh parallel downloads, sequential transfers with safe resume, and local checkpoint inspection. Parallel partial resume and retries remain planned.

## Build locally

Use Go 1.27 or newer and GNU Make. From the repository root:

```sh
make build
./bin/yoinkdl version
```

The executable is `bin/yoinkdl`; the default version is `dev`. This is a source build, not a published release or package-manager installation.

Linux is tested locally. macOS storage operations are enabled but need native validation; Windows downloads are unsupported. The destination filesystem must support advisory locks, hard links, and directory sync.

Run tests or the full gate:

```sh
make test
make check
```

Tests use local HTTP servers. `make check` also checks formatting, runs vet and race tests, and builds. See [development](development.md) for all targets.

## Download a file

Example URLs are placeholders. Replace them with an accessible direct file URL:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip'
```

The default destination uses a safe filename from the URL path in the current directory, falling back to `download`. Query strings do not become filenames.

Create a destination directory yourself when needed:

```sh
mkdir -p downloads
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./downloads/archive.zip
```

The CLI does not create missing parent directories. Quote URLs, especially when they contain query parameters.

Downloads use up to four workers when safe ranges are supported. Choose a limit from 1 to 32 with `--connections` or `-c`. Starting with `--connections 1` uses a direct sequential stream and the currently implemented resume path.

## Read progress and results

Known-size downloads show bytes, percentage, average speed, ETA, and active workers. Unknown-size downloads show bytes and speed. Progress goes to stderr; success prints the absolute final path to stdout.

Use quiet mode for less terminal output:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --quiet
```

A stalled connection or body read fails after the configured inactivity timeout. To use two minutes:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --timeout 2m
```

Active downloads may take longer than that timeout overall.

## Interrupt a transfer

Press Ctrl+C. The process cancels requests, waits for workers to stop, preserves safe checkpoints, and reports the retained partial file. Interruption exits with status `130` and does not publish an incomplete output. Parallel transfers checkpoint only complete validated chunks.

A typical retained artifact is:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

For compatible sequential checkpoints, repeat the same URL and output to resume automatically, or require saved state explicitly:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --resume
./bin/yoinkdl inspect ./archive.zip
```

Resume requires a known total, identity encoding, a matching strong ETag, and valid local state. Inspection only shows saved metadata; the download performs the checks before reuse. Servers without safe validators/ranges and legacy partials without a manifest cannot resume.

Parallel partial checkpoints remain preserved and inspectable, but their network resume is not implemented yet. Use `--connections 1` from the start when the transfer needs resume support in this development build. That flag does not convert existing parallel state.

After a crash, the last validated checkpoint may be behind displayed progress. Recovery hashes the saved prefix before network reuse, which can take time for a large file, and re-fetches uncertain bytes. A lock file can remain after process death or success; the OS releases ownership automatically, so do not delete a lock merely because its file remains.

Use a fresh output path when state is incompatible, or deliberately abandon old artifacts only after their writer has stopped. Do not rename a partial file into a final output manually.

## Replace a completed output

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip --overwrite
```

This allows replacing an existing regular file after the transfer verifies successfully. It can also accompany compatible resume, but never discards incompatible state or follows destination symlinks.

## Current limits

Parallel partial resume and automatic retries remain unimplemented. `--retries`, `--limit`, and `--json` are not available yet. Configuration files are not loaded.

The engine asks for identity encoding and saves the exact bytes the server returns. If the server still applies content encoding, those bytes are preserved without transparent decoding. Unknown-length completion can establish clean HTTP completion, not the publisher's intended size without additional metadata.

See the [CLI reference](cli.md), [troubleshooting](troubleshooting.md), and [roadmap](roadmap.md).
