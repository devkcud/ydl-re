# YoinkDL

*The download manager that yoinks.*

YoinkDL is a planned local-first download manager written in Go. It aims to download HTTP(S) files reliably, preserve safe progress through interruptions, and resume without silently mixing different versions of a file.

**Yoink a file. Keep it intact. Pick up where you left off.**

## Status

The project currently contains its design, documentation, and contributor workflow. Implementation has not started: there is no Go module, executable, test suite, or verified installation method yet.

Read [STATUS.md](STATUS.md) for the current handoff and next implementation steps. Command examples below describe the planned interface.

## Planned first release

- Standalone `download`, `inspect`, and `version` commands.
- Direct HTTP(S) downloads with safe sequential fallback and bounded parallel ranges.
- Local manifests and checkpoints for compatible resume after interruption.
- Retries, progress, an aggregate transfer speed cap, and machine-readable output.
- Completion checks, exclusive transfer ownership, and verified file publication.

The first release requires no daemon or database. A persistent queue, local daemon, and TUI belong to later releases. See the [roadmap](docs/roadmap.md) for boundaries and acceptance criteria.

## Planned usage

These commands will become available after implementation and installation. Replace the placeholder URL with a direct file URL you can access.

```sh
yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip
yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip --resume
yoinkdl inspect ./archive.zip
yoinkdl version
```

Resumption requires compatible local state and a trustworthy match to the remote resource version. Existing completed files are preserved unless replacement is explicitly requested.

## Documentation

| Resource | Purpose |
|---|---|
| [Documentation index](docs/README.md) | All project guides |
| [Getting started](docs/getting-started.md) | Availability and the intended download workflow |
| [CLI reference](docs/cli.md) | Proposed commands and options |
| [Configuration](docs/configuration.md) | Defaults, units, and local paths |
| [Architecture](docs/architecture.md) | Engine components and responsibilities |
| [Resume and storage](docs/resume-and-storage.md) | State, checkpoints, locking, and recovery |
| [Development](docs/development.md) | Build workflow and contribution guidance |
| [Testing](docs/testing.md) | Required correctness and failure scenarios |
| [Roadmap](docs/roadmap.md) | Milestones and release scope |
| [Original design](yoinkdl-design.md) | Engineering proposal |

## Contributing

Read [AGENTS.md](AGENTS.md) and [STATUS.md](STATUS.md) before making changes. Start with the first unfinished next step and follow the roadmap's dependency order.

After every implementation step, update `STATUS.md` with a handoff, validation results, open issues, and ordered next steps. Keep the status current even when a step is incomplete or a check fails.

The first milestone is a reliable single-stream CLI. Build and test instructions in the [development guide](docs/development.md) are templates until the Go source exists.

## License

An open-source license has not been selected and no `LICENSE` file exists yet.
