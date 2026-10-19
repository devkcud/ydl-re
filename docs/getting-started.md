# Getting started

[Documentation index](README.md)

The CLI supports parallel and sequential downloads with safe resume, bounded retries, aggregate rate caps, and local checkpoint inspection.

Linux/amd64 users can [install stable `0.1.0`](releases.md#install-a-published-binary). The steps below build from source and use `./bin/yoinkdl`; with an installed binary on PATH, use `yoinkdl` instead.

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

Downloads use up to four workers when safe ranges are supported. Choose a limit from 1 to 32 with `--connections` or `-c`. One connection uses a direct stream for a new file or resumes saved chunks with one worker.

Cap the combined body rate of all workers:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --limit 5MB/s
```

`5MB/s` means 5,000,000 bytes per second; `5MiB/s` means 5,242,880. Rates require positive integers and explicit units. Omit the option for unlimited transfers. Probe/retry bytes count; protocol overhead and socket read-ahead are outside the cap. Throttle waits honor Ctrl+C and do not trigger the network inactivity timeout.

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

Transient requests and incomplete range chunks get three additional attempts by default. Set `--retries 0` to disable retries, or choose a decimal count up to 10. Backoff and reasonable `Retry-After` waits are cancellable. A retry replaces the whole failed chunk, so displayed progress may briefly decrease. Sequential body interruptions preserve checkpoints for a later resume.

For scripts, use `--json` to receive one JSON object per line on stdout. Combine it with `--quiet` for just the terminal result:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --json --quiet
```

Success requires `status: "completed"`, `published: true`, and exit status `0`. A failed/interrupted result reports known retained partial data. Output failures use stderr and a nonzero exit; stdout may then contain an incomplete line or no terminal result. See [JSON output](cli.md#json-output) for the schema, unknown totals, and stream behavior.

## Interrupt a transfer

Press Ctrl+C. The process cancels requests, waits for workers to stop, preserves safe checkpoints, and reports the retained partial file. Interruption exits with status `130` and does not publish an incomplete output. Parallel transfers checkpoint only complete validated chunks.

A typical retained artifact is:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

For compatible checkpoints, repeat the same URL and output to resume automatically, or require saved state explicitly:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --resume
./bin/yoinkdl inspect ./archive.zip
```

Resume requires a known total, identity encoding, a matching strong ETag, and valid local state. Inspection only shows saved metadata; the download performs the checks before reuse. Servers without safe validators/ranges and legacy partials without a manifest cannot resume.

Parallel resume validates the saved version with the server before fetching whole missing chunks. Completed chunks are reused, and the saved chunk size stays fixed. You can change `--connections` between attempts without changing the checkpoint format.

After a crash, the last validated checkpoint may be behind displayed progress. Recovery hashes saved prefix or chunk bytes before network reuse, which can take time for a large file, and re-fetches uncertain work. A lock file can remain after process death or success; the OS releases ownership automatically, so do not delete a lock merely because its file remains.

Use a fresh output path when state is incompatible, or deliberately abandon old artifacts only after their writer has stopped. Do not rename a partial file into a final output manually.

## Replace a completed output

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip --overwrite
```

This allows replacing an existing regular file after the transfer verifies successfully. It can also accompany compatible resume, but never discards incompatible state or follows destination symlinks.

## Current limits

Configuration files are not loaded. Automatic sequential body restart is not implemented; repeat the same request to resume compatible saved state.

The engine asks for identity encoding and saves the exact bytes the server returns. If the server still applies content encoding, those bytes are preserved without transparent decoding. Unknown-length completion can establish clean HTTP completion, not the publisher's intended size without additional metadata.

See the [CLI reference](cli.md), [troubleshooting](troubleshooting.md), and [roadmap](roadmap.md).

## Use the Linux queue

The repository build also provides [v0.2 development queue commands](cli.md#daemon-and-queue), beyond published `0.1.0`. Run `yoinkdl daemon start` explicitly in one terminal, then use add/list/control commands from another terminal with the same XDG environment. Add resolves outputs in the client's directory and prints a job ID; successful admission does not mean the file has completed. A lost reply retains that ID for inspection, and broken client output never cancels durable jobs. Final v0.2 process/release qualification remains pending.
