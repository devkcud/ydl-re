# Development guide

[Documentation index](README.md)

Implementation has not started in this checkout. This guide separates what contributors can review today from the build workflow to adopt once source code exists.

## Current checkout

```text
yoink-reborn/
├── AGENTS.md
├── README.md
├── STATUS.md
├── docs/
└── yoinkdl-design.md
```

The directory is named `yoink-reborn`; the proposed product and executable are `YoinkDL` and `yoinkdl`. There is no module path, minimum Go version, source tree, test suite, CI workflow, or license file yet.

Start with [AGENTS.md](../AGENTS.md) and the current handoff in [STATUS.md](../STATUS.md), then read the relevant [design](../yoinkdl-design.md), [architecture](architecture.md), and [roadmap](roadmap.md) sections. The first implementation task is Milestone A: a reliable single stream.

## Before scaffolding

Choose the Go version and module path, then create the smallest structure needed for `download` and `version`. Select a license before distributing the project as open source. Record decisions in the relevant documentation.

Do not create every proposed package just to match a diagram. Add boundaries as responsibilities emerge. Do not add a daemon, database, framework, or workflow runtime to bootstrap a file transfer.

Dependencies should be justified against the standard library. CLI parsing and TOML loading may use small libraries if chosen; no particular dependency is required by the current design.

## Planned build workflow

These commands are templates for the [proposed source layout](architecture.md#proposed-repository-layout). They do not work in the current documentation-only checkout.

After the module, entry point, and source packages exist:

```sh
go build -o ./bin/yoinkdl ./cmd/yoinkdl
```

Choose an `.exe` output name for a Windows binary. Use the toolchain selected in `go.mod`; this documentation does not prescribe an unverified version.

Run the entry point during development:

```sh
go run ./cmd/yoinkdl version
```

Run the future verification commands:

```sh
go test ./...
go vet ./...
go test -race ./...
```

Race testing requires a supported environment. Cross-compiling alone cannot validate signal behavior, filesystem locking, or publication guarantees on another operating system.

## Working on a change

1. Identify the milestone and behavior the change is meant to deliver.
2. Check existing code and local changes before editing.
3. Add the smallest responsible implementation and meaningful tests for its behavior.
4. Exercise both the success path and relevant interruption/error paths.
5. Run focused checks, then the applicable project-wide checks.
6. Update the affected docs with implemented behavior and remaining limitations.
7. Update the root [STATUS.md](../STATUS.md) after every implementation step with the handoff, validation results, open issues, and ordered next steps. Include incomplete work and failed or skipped checks before ending the session.

Keep unrelated work intact. A documentation update or proposed interface is not evidence that the feature works. Keep `STATUS.md` a current snapshot and remove stale next steps as work is completed.

## Code conventions

- Keep explanations, names, and functions clear and concise.
- Use short variable names that still explain their role.
- Do not prefix private identifiers with `_` unless an established project convention requires it.
- Make visibility deliberate. In Go, exported identifiers use an uppercase initial; internal identifiers use a lowercase initial. Go has no explicit visibility modifiers.
- Use `const` for values that are valid constants. Other bindings should have the narrowest useful scope.
- Keep behavior that does not depend on an instance in an appropriate package function or type-level construct supported by the language.
- Avoid comments unless logic is extremely complex or cannot be made clear in the code itself.
- Format Go source with `gofmt` once it exists.

The project does not yet have a selected linter, coverage threshold, public Go API policy, or mandatory third-party testing framework.

## Test design

Use deterministic fixtures and local HTTP servers for correctness. Inject failures where the engine must preserve invariants: wrong ranges, resource changes, incomplete reads, failed writes, interrupted checkpoints, and concurrent ownership attempts.

Do not depend on public download URLs for repeatable tests. Hash fixture output in integration tests even though a user-facing checksum flag is deferred. See the [full test plan](testing.md).

## Documentation maintenance

Keep user commands in [CLI](cli.md), defaults and paths in [configuration](configuration.md), and recovery guarantees in [resume and storage](resume-and-storage.md). Update cross-references when moving a page.

When a decision is resolved, update the owning page and remove or mark the corresponding item in [open decisions](decisions.md). When implementation arrives, replace planned examples with commands verified against it and update [project status](README.md#project-status).

Check Markdown links, anchors, code fences, structured examples, and whitespace. For new untracked pages, use checks that read their contents; a plain `git diff --check` does not include them.

## Release preparation

Milestone D includes native platform checks, installation instructions, release binaries, and a changelog. The distribution mechanism, signing/checksum policy for artifacts, version injection, and CI provider are not decided.

Tagging a release requires the [acceptance criteria](roadmap.md#v010-acceptance) and actual test evidence. No build, integration test, race check, or platform qualification is claimed by this documentation work.
