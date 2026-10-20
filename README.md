# YoinkDL

*The download manager that yoinks.*

YoinkDL is a local-first HTTP(S) download manager written in Go. The first implementation provides a foreground CLI that streams into a separate partial file, checks completion, and safely publishes the result.

## Status

Milestones A through D are complete for the initial Linux/amd64 release. `download`, `inspect`, `version`, safe resume, bounded retries, progress, cancellation, overwrite protection, and inactivity timeouts work. Saved progress is synced and checksummed before reuse.

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) is the latest stable release. It includes aggregate rate caps, versioned JSON output, checked text/terminal output, a completed local [security review](docs/security.md), and passing hosted CI. Public downloads, checksums, source identity, and HTTP/HTTPS installation tests passed. Native qualification beyond Linux/amd64 remains open. Read [STATUS.md](STATUS.md) for the handoff and next steps.

## Build and test

Install Go 1.27 or newer and GNU Make, then run from the repository root:

```sh
make build
./bin/yoinkdl version
```

The executable is written to `bin/yoinkdl`. No external Go dependencies or services are required.

```sh
make test
make check
```

`make test` runs the tests. `make check` checks formatting, runs vet, tests and the race detector, then builds. Tests use localhost servers and require permission to open local sockets; race checks require a supported Go/CGO environment.

Other targets are `make vet`, `make race`, `make fmt`, `make fmt-check`, and `make help`. Set a build label with `make build VERSION=0.1.0-dev`.

On native Linux/amd64, packaging tests also require Bash, Git, GNU tar/coreutils, and gzip. From a clean committed tree, `make release VERSION=0.1.0` runs the gate and creates binary/source archives and checksums under `bin/releases/`. Packaging does not create a tag or publish a release. To use the stable release, follow the [binary installation instructions](docs/releases.md#install-a-published-binary) and its [release notes](CHANGELOG.md#010).

## Download

Replace the placeholder with a direct file URL:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip
```

The parent directory must already exist. By default, existing completed files and partial state are preserved. Use `--overwrite` only to replace a completed output deliberately:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --overwrite
```

Use `--connections <n>` or `-c <n>` to choose a worker limit from 1 to 32. One connection uses a direct stream for a new file, or one worker when resuming saved chunks. Use `--quiet` to suppress progress and `--timeout 2m` to change the network inactivity timeout. Progress and errors go to stderr; success prints the absolute output path to stdout.

`--retries <n>` accepts 0 through 10 additional attempts; zero disables retries. Incomplete ranges restart from their assigned beginning after cancellable backoff. Interrupted sequential bodies keep their checkpoints for a later resume.

Use `--limit 5MB/s` to cap body traffic across all workers at 5,000,000 bytes per second. `5MiB/s` uses binary units. Rates require positive integers and explicit units; omission is unlimited. Probe and retry bytes count, throttle waits are cancellable, and protocol overhead/read-ahead is outside this application-level cap. See [rate accounting](docs/concurrency.md#aggregate-rate-cap).

Use `--json` for newline-delimited progress and a terminal result on stdout. `--json --quiet` emits only the result. Check both the result and exit status; failures can preserve partial data or occur after publication. See [the JSON contract](docs/cli.md#json-output).

Text output escapes nonprinting characters in paths without changing the filename. Failed progress output stops the transfer; failure to print a final result leaves any verified published file intact. Pipe/socket/terminal writes have bounded waits, and interruption still performs transfer cleanup.

Ctrl+C stops all workers and preserves verified checkpoints. Run the same URL and output again to resume compatible sequential or parallel state automatically. Parallel resume checks the saved version with the server before fetching whole missing chunks; completed chunks are retained.

Require an existing compatible checkpoint explicitly:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --resume
./bin/yoinkdl inspect ./archive.zip
```

Partial reuse requires a known size, identity encoding, and a matching strong ETag. Changed URLs/versions, corrupt state, and legacy partials without manifests are refused without destructive restart. A small `.yoinkdl.lock` file remains after success; its presence alone does not mean a process is active.

## Linux queue (0.2.0-rc.1 prerelease)

[YoinkDL 0.2.0-rc.1](https://github.com/devkcud/ydl-re/releases/tag/v0.2.0-rc.1) is published for Linux/amd64 with explicit daemon and queue commands. E1 through E6 pass local and hosted qualification, including native ordinary/race/CGO-disabled gates, six-target compilation, and [public archive/installation checks](docs/releases.md#v02-hosted-qualification-and-publication). Stable `0.1.0` remains Latest and provides standalone downloads.

Run the owner in one terminal:

```sh
./bin/yoinkdl daemon start --parallel 2 --limit 5MB/s
```

In another terminal with the same XDG environment:

```sh
id=$(./bin/yoinkdl add 'https://example.org/archive.zip' -o ./archive.zip)
./bin/yoinkdl list --json
./bin/yoinkdl pause "$id"
./bin/yoinkdl resume "$id"
./bin/yoinkdl daemon stop
```

Clients never start a daemon implicitly. Add returns a job ID after durable admission; output failures do not cancel accepted jobs. A lost acknowledgment reports an unknown outcome and retains the submitted ID. See [commands and JSON results](docs/cli.md#daemon-and-queue) for controls, source privacy, and output behavior.

## Platforms

Linux/amd64 has local runtime evidence on ext4 and tmpfs, including race tests on ext4, plus passing hosted Ubuntu 24.04.5 checks with Go 1.27.0. CLI builds and test compilation pass for Linux, macOS, and Windows on amd64/arm64. Linux/arm64 and macOS still need native qualification; Windows storage remains explicitly unsupported. See the [platform matrix and reproduction commands](docs/platforms.md).

The output filesystem must support advisory locking, hard links, and directory sync. Network filesystems and power-loss durability remain unqualified.

## Documentation

| Resource | Purpose |
|---|---|
| [Documentation index](docs/README.md) | All project guides |
| [Getting started](docs/getting-started.md) | Build, download, and interruption workflow |
| [CLI reference](docs/cli.md) | Implemented commands and planned additions |
| [Configuration](docs/configuration.md) | Current defaults and proposed configuration |
| [Architecture](docs/architecture.md) | Current packages and target design |
| [Daemon and queue](docs/daemon-and-queue.md) | Linux v0.2 prerelease daemon, queue commands, private IPC, and shared caps |
| [Initial TUI](docs/tui.md) | Planned v0.3 terminal interface, implementation steps, and acceptance criteria |
| [Resume and storage](docs/resume-and-storage.md) | File safety, checkpoints, and recovery |
| [Development](docs/development.md) | Make targets and contributor workflow |
| [Testing](docs/testing.md) | Current evidence and remaining scenarios |
| [Platforms](docs/platforms.md) | Native evidence, cross-build matrix, and support limits |
| [Releases](docs/releases.md) | CI, packaging, draft releases, and binary installation |
| [Roadmap](docs/roadmap.md) | Milestones and release scope |
| [Original design](yoinkdl-design.md) | Unmodified engineering proposal |

## Contributing

Read [AGENTS.md](AGENTS.md) and [STATUS.md](STATUS.md). Follow the first unfinished next step, keep code clearly spaced, and update the status handoff after every implementation step. Split changes into focused local commits and never push.

## License

This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

The repository-wide license is [MPL-2.0](LICENSE). Report security problems through the public channel described in [SECURITY.md](SECURITY.md).
