# Getting started

[Documentation index](README.md)

This guide describes the planned v0.1 workflow. There is no runnable YoinkDL implementation in this checkout yet.

## Installation status

The repository has no `go.mod`, source packages, release binaries, or installer configuration. A Go version, module path, distribution channel, and license still need to be chosen.

There is no verified `go install`, package-manager, or binary-download command to use today. Contributors can start with the [development guide](development.md) and [Milestone A](roadmap.md#milestone-a--reliable-single-stream).

The following examples become usable after the CLI is implemented and installed. `example.org` is a placeholder; replace it with a direct file URL you can access.

## Download a file

```sh
yoinkdl download 'https://example.org/archive.zip'
```

The intended default is to save into the current directory using a safe filename derived from the URL path. When no usable name exists, use `download`.

Choose a destination explicitly:

```sh
yoinkdl download 'https://example.org/archive.zip' --output ./downloads/archive.zip
```

Use an existing writable parent directory until automatic directory creation is specified. Quote URLs, particularly when they contain query parameters.

## Control parallelism and speed

```sh
yoinkdl download 'https://example.org/archive.zip' --connections 4 --limit 5MB/s
```

Four connections is a desired worker count. The engine may use fewer workers for a small file or a server that cannot support safe ranged downloads.

`5MB/s` means an aggregate cap of 5,000,000 bytes per second across the transfer's workers. The cap is optional; the default is unlimited. See [configuration and units](configuration.md#units).

## Interrupt and resume

Interrupt the foreground process with Ctrl+C. The planned shutdown cancels requests and attempts to preserve a safe checkpoint. A forced termination may lose progress since the last durable checkpoint.

Run the same request with the same output path:

```sh
yoinkdl download 'https://example.org/archive.zip' --output ./downloads/archive.zip --resume
```

Compatible partial state is intended to resume automatically, with `--resume` also supported explicitly. Reuse depends on the remote resource version, destination, manifest, and partial file agreeing. A changed resource or incompatible state must not be silently combined with old bytes.

The design does not yet fix the restart interaction for every incompatible case. A fresh output path is the conservative way to start a separate transfer while preserving existing artifacts.

## Inspect local state

```sh
yoinkdl inspect ./downloads/archive.zip
```

`inspect` is intended to show locally stored transfer metadata for that destination. It is not a promise of completed-download history: the successful transfer normally removes its metadata. The output layout and behavior when metadata is absent are still open.

Typical planned artifacts during a transfer are:

```text
downloads/
├── archive.zip.yoinkdl.part
└── archive.zip.yoinkdl.json
```

Successful finalization publishes `archive.zip` and cleans up transfer metadata when safe. Keep the partial file and manifest together while recovery may still be needed. See [resume and storage](resume-and-storage.md).

## Replace a completed output

```sh
yoinkdl download 'https://example.org/archive.zip' --output ./downloads/archive.zip --overwrite
```

`--overwrite` deliberately allows replacing a completed destination. Without it, a destination collision must fail. This flag is not permission to merge incompatible partial state or skip verification.

## Read progress and results

For a known total, the terminal should show transferred bytes, percentage, speed, useful ETA, and active workers. For an unknown total, it should show bytes and speed without inventing a percentage or ETA.

Success prints the final path and exits with `0`. Failure exits nonzero, explains the reason, and states whether partial data was preserved. Exact text and JSON formats are not frozen.

See the [CLI reference](cli.md) for all proposed flags and [troubleshooting](troubleshooting.md) for expected recovery situations.
