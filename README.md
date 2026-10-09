# YoinkDL

*The download manager that yoinks.*

YoinkDL is a local-first HTTP(S) download manager written in Go. The first implementation provides a foreground CLI that streams into a separate partial file, checks completion, and safely publishes the result.

## Status

Milestone A is implemented and tested on Linux. `download`, `version`, progress, cancellation, overwrite protection, and network inactivity timeouts work. Nonempty partial files are preserved after failures or interruption.

Resume, `inspect`, parallel ranges, retries, bandwidth caps, JSON output, and configuration files are still planned. This is a development build, not the complete v0.1 release. Read [STATUS.md](STATUS.md) for the handoff and next steps.

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

## Download

Replace the placeholder with a direct file URL:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip
```

The parent directory must already exist. By default, existing completed files and partial state are preserved. Use `--overwrite` only to replace a completed output deliberately:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --overwrite
```

Use `--quiet` to suppress progress and `--timeout 2m` to change the network inactivity timeout. Progress and errors go to stderr; success prints the absolute output path to stdout. Run `./bin/yoinkdl download --help` for all implemented options.

Ctrl+C stops the transfer and retains nonempty `.yoinkdl.part` data. Resume is not implemented yet: use a fresh output path for a new attempt, or deliberately remove abandoned partial data only after its writer has stopped.

## Platforms

Linux is tested locally. Storage operations are also enabled on macOS, which still needs native qualification. Windows downloads are explicitly unsupported until safe publication is implemented there. The output filesystem must support hard links and directory sync.

## Documentation

| Resource | Purpose |
|---|---|
| [Documentation index](docs/README.md) | All project guides |
| [Getting started](docs/getting-started.md) | Build, download, and interruption workflow |
| [CLI reference](docs/cli.md) | Implemented commands and planned additions |
| [Configuration](docs/configuration.md) | Current defaults and proposed configuration |
| [Architecture](docs/architecture.md) | Current packages and target design |
| [Resume and storage](docs/resume-and-storage.md) | File safety and planned recovery |
| [Development](docs/development.md) | Make targets and contributor workflow |
| [Testing](docs/testing.md) | Current evidence and remaining scenarios |
| [Roadmap](docs/roadmap.md) | Milestones and release scope |
| [Original design](yoinkdl-design.md) | Unmodified engineering proposal |

## Contributing

Read [AGENTS.md](AGENTS.md) and [STATUS.md](STATUS.md). Follow the first unfinished next step, keep code clearly spaced, and update the status handoff after every implementation step. Split changes into focused local commits and never push.

## License

An open-source license has not been selected and no `LICENSE` file exists yet.
