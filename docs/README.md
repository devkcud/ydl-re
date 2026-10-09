# YoinkDL documentation

*The download manager that yoinks.*

YoinkDL is a proposed local-first download manager written in Go. Its first release focuses on reliable HTTP(S) downloads from a standalone CLI, with safe interruption, resumption, and file completion.

## Project status

As of October 9, 2026, this checkout contains the [engineering design](../yoinkdl-design.md), documentation, and contributor workflow. It has no Go module, executable implementation, tests, or release tooling yet.

Use the root [README](../README.md) for an introduction, [STATUS.md](../STATUS.md) for the current handoff and next steps, and [AGENTS.md](../AGENTS.md) for implementation instructions.

Command examples describe the planned interface. Package layouts, configuration, and state examples describe the design, not working software. No installation method is verified yet.

These pages expand the original design without marking proposed features as delivered. Requirements describe the intended product; recommendations and unresolved details are identified explicitly. The [open decisions](decisions.md) must be settled as implementation proceeds.

## Start here

- **Understand the project:** read the [overview](overview.md) and [roadmap](roadmap.md).
- **Learn the intended workflow:** read [getting started](getting-started.md), the [CLI reference](cli.md), and [troubleshooting](troubleshooting.md).
- **Build the engine:** read [architecture](architecture.md), [transfer lifecycle](transfer-lifecycle.md), [HTTP handling](http.md), and [resume and storage](resume-and-storage.md).
- **Contribute:** start with [agent instructions](../AGENTS.md) and the [current handoff](../STATUS.md), then read [development](development.md), [testing](testing.md), and [open decisions](decisions.md).

## Documentation map

| Page | Covers |
|---|---|
| [Overview](overview.md) | Product goals, release scope, non-goals, terminology |
| [Getting started](getting-started.md) | Availability, planned first download, interruption and recovery |
| [CLI reference](cli.md) | Commands, flags, output, exit behavior |
| [Configuration](configuration.md) | Defaults, proposed TOML, paths, units |
| [Architecture](architecture.md) | Components, package boundaries, future front ends |
| [Transfer lifecycle](transfer-lifecycle.md) | States, verification, final publication |
| [HTTP handling](http.md) | Probing, ranges, validators, fallback, redirects |
| [Resume and storage](resume-and-storage.md) | Sidecars, manifest, checkpoints, locks, crash recovery |
| [Concurrency and retries](concurrency.md) | Range planning, workers, backoff, rate limits, progress |
| [Security](security.md) | File safety, credentials, network trust, future daemon risks |
| [Development](development.md) | Current checkout, proposed build workflow, coding conventions |
| [Testing](testing.md) | Deterministic tests, fault cases, release evidence |
| [Troubleshooting](troubleshooting.md) | Expected failures and safe recovery guidance |
| [Roadmap](roadmap.md) | Milestones, acceptance criteria, deferred features |
| [Open decisions](decisions.md) | Unsettled interfaces and implementation policies |

## Design source map

The original design remains the source proposal. This map shows where its subjects are explained in detail.

| Original section | Documentation |
|---|---|
| 1. Summary | [Overview](overview.md) |
| 2. Release boundaries | [Overview](overview.md), [roadmap](roadmap.md) |
| 3. Product behavior and CLI | [Getting started](getting-started.md), [CLI](cli.md) |
| 4. Architecture | [Architecture](architecture.md), [development](development.md) |
| 5. Download lifecycle | [Lifecycle](transfer-lifecycle.md), [HTTP](http.md) |
| 6. Resume and state model | [Resume and storage](resume-and-storage.md) |
| 7. Concurrency, retries, and limits | [Concurrency](concurrency.md) |
| 8. State and configuration paths | [Configuration](configuration.md) |
| 9. Security and file safety | [Security](security.md), [troubleshooting](troubleshooting.md) |
| 10. Testing strategy | [Testing](testing.md) |
| 11. Concrete implementation sequence | [Roadmap](roadmap.md), [development](development.md) |
| 12. Later architecture | [Architecture](architecture.md), [roadmap](roadmap.md) |
| 13. Deferred original concepts | [Roadmap](roadmap.md) |
| 14. Definition of success | [Overview](overview.md), [testing](testing.md) |

## Documentation conventions

`v0.1`, `v0.2`, and `v0.3` identify planned release boundaries, not existing releases. “Later” means unscheduled. “Illustrative” means an example is not a stable schema or output contract.

All example URLs and filesystem paths are placeholders. A successful-looking progress display is never evidence that a transfer has passed verification.
