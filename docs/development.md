# Development guide

[Documentation index](README.md)

The repository contains the Milestone A/B CLI, persistent storage, and Milestone C parallel transfers with safe resume. Read [AGENTS.md](../AGENTS.md) and the current handoff in [STATUS.md](../STATUS.md) before making changes.

## Current checkout

```text
yoink-reborn/
├── AGENTS.md
├── Makefile
├── README.md
├── STATUS.md
├── cmd/yoinkdl/
├── docs/
├── internal/
│   ├── cli/
│   ├── ranges/
│   ├── storage/
│   └── transfer/
├── go.mod
└── yoinkdl-design.md
```

The module path is `yoinkdl`, with Go 1.27 as the minimum selected toolchain. All dependencies are from the standard library. The local module name does not imply a published remote package.

The original [architecture](architecture.md) also describes future boundaries. Add packages only when a distinct responsibility needs them; do not scaffold every proposed component in advance.

## Build and check

Use GNU Make from the repository root:

```sh
make build
./bin/yoinkdl version
```

The ignored `bin/` directory contains build output. The default version label is `dev`; `make build VERSION=0.1.0-dev` sets another label without creating a release tag.

| Target | Action |
|---|---|
| `make`, `make build` | Build the executable with trimmed source paths |
| `make test` | Run `go test ./...` |
| `make vet` | Run `go vet ./...` |
| `make race` | Run `go test -race ./...` |
| `make fmt` | Run `gofmt -w cmd internal` |
| `make fmt-check` | Fail if Go source needs formatting |
| `make check` | Formatting check, vet, tests, race tests, then build |
| `make help` | List targets |

Go's build and test caches are used normally. The Makefile does not clear them. A direct build also works:

```sh
go build -o ./bin/yoinkdl ./cmd/yoinkdl
```

## Test environment

Tests use localhost HTTP servers, temporary directories, and subprocesses. Sandboxes must allow local socket listeners. The race detector requires a supported Go/CGO environment.

The implementation was checked with Go 1.27.2 on Linux/amd64. Initial sandboxed Go checks could not resolve standard packages, and HTTP tests could not open sockets; equivalent checks outside the sandbox passed. These are environment restrictions, not skipped correctness cases.

Only Linux has native runtime evidence. macOS code paths are enabled but unqualified; Windows downloads are rejected until publication is implemented there. Cross-compiling would not establish filesystem or signal guarantees.

## Working on a change

1. Resume the first unfinished next step in `STATUS.md` within the user's requested scope.
2. Inspect local changes and the relevant contracts before editing.
3. Implement one coherent behavior with meaningful success and failure tests.
4. Keep code clearly spaced, following `gofmt` and the [agent rules](../AGENTS.md).
5. Run focused checks, then the applicable full gate.
6. Update the owning docs and the handoff, validation, open issues, and next steps in `STATUS.md`.
7. Stage explicit paths or hunks and split focused commits by responsibility and dependency order. Never push.

Update the handoff after every implementation step, even when work is incomplete or a check fails. Keep unrelated changes intact and replace stale next steps instead of accumulating a session log.

## Code and package boundaries

`cmd/yoinkdl` owns process signals and the entry point. `internal/cli` owns parsing, local inspection, stdout/stderr, and progress rendering. `internal/transfer` owns request/response identity, resume validation, transfer completion, and snapshots. `internal/ranges` owns chunk arithmetic and coverage. `internal/storage` owns stable locks, versioned manifest validation, prefix/chunk hashing, checkpoints, and publication recovery.

Use short, clear names, explicit visibility where supported, constants for fixed values, and blank lines between logical blocks. Go uses capitalization for visibility and package-level functions instead of class-level static methods. Avoid comments unless complex logic cannot be made clear in code.

Keep the transfer/storage packages independent of terminal rendering. Use deterministic fixtures and local servers for correctness, including interruption, malformed responses, resource conflicts, and failed publication.

## Documentation maintenance

Keep current commands in [CLI](cli.md), defaults in [configuration](configuration.md), and file guarantees in [resume and storage](resume-and-storage.md). Mark future interfaces as planned and update [open decisions](decisions.md) when choices are adopted.

Check local links, anchors, code fences, structured examples, shell syntax, and whitespace. Checks must include untracked files; `git diff --check` alone does not inspect them. The original design contains intentional Markdown hard-break spaces and remains unchanged.

## Release preparation

Milestone D is in progress: aggregate rate caps, JSON events, text-output handling, and security regression fixes have local Linux evidence. The [security review](security.md) records the tested boundaries; platform/release qualification remains. There is no license, CI workflow, installer, or published release configuration yet. Select those before distribution and validate every claimed platform natively.

Tag v0.1.0 only when the [release acceptance criteria](roadmap.md#v010-acceptance) have actual evidence. Milestone B success does not qualify parallel ranges, retry policies, other platforms, or the full v0.1 feature set.
