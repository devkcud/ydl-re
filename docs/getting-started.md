# Getting started

[Documentation index](README.md)

Milestones A and B provide a working sequential download CLI with safe resume and local inspection. Parallel transfers are still planned.

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

## Read progress and results

Known-size downloads show bytes, percentage, average speed, ETA, and one worker. Unknown-size downloads show bytes and speed. Progress goes to stderr; success prints the absolute final path to stdout.

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

Press Ctrl+C. The process cancels the request, saves safe progress, closes received data, and reports the retained partial file. Interruption exits with status `130` and does not publish an incomplete output.

A typical retained artifact is:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

Repeat the same URL and output to resume compatible state automatically, or require a saved transfer explicitly:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --resume
./bin/yoinkdl inspect ./archive.zip
```

Resume requires a known total, identity encoding, a matching strong ETag, and valid local state. Inspection only shows saved metadata; the download performs the checks before reuse. Servers without safe validators/ranges and legacy partials without a manifest cannot resume.

After a crash, the last validated checkpoint may be behind displayed progress. Recovery hashes the saved prefix before network reuse, which can take time for a large file, and re-fetches uncertain bytes. A lock file can remain after process death or success; the OS releases ownership automatically, so do not delete a lock merely because its file remains.

Use a fresh output path when state is incompatible, or deliberately abandon old artifacts only after their writer has stopped. Do not rename a partial file into a final output manually.

## Replace a completed output

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip --overwrite
```

This allows replacing an existing regular file after the transfer verifies successfully. It can also accompany compatible resume, but never discards incompatible state or follows destination symlinks.

## Current limits

The engine uses one stream and no automatic retries. `--connections`, `--retries`, `--limit`, and `--json` are not available yet. Configuration files are not loaded.

The engine asks for identity encoding and saves the exact bytes the server returns. If the server still applies content encoding, those bytes are preserved without transparent decoding. Unknown-length completion can establish clean HTTP completion, not the publisher's intended size without additional metadata.

See the [CLI reference](cli.md), [troubleshooting](troubleshooting.md), and [roadmap](roadmap.md).
