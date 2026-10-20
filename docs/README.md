# YoinkDL documentation

*The download manager that yoinks.*

YoinkDL is a local-first download manager written in Go. It provides sequential and bounded parallel HTTP(S) transfers with safe resumption, bounded retries, aggregate rate limits, and versioned JSON output.

## Project status

As of October 19, 2026, stable `0.1.0` is published for Linux/amd64. The transfer, resume, and retry engine has native ext4/tmpfs evidence, hosted v0.1 CI, and six-target compilation evidence. Use `make build` to build `bin/yoinkdl`, `make test` for tests, and `make check` for the full gate. There are no external Go dependencies.

Use the root [README](../README.md) for an introduction, [STATUS.md](../STATUS.md) for the current handoff and next steps, and [AGENTS.md](../AGENTS.md) for implementation instructions.

The [getting-started guide](getting-started.md) and [CLI reference](cli.md) document working commands, including connection/retry limits, aggregate rate caps, JSON events, resume, and local inspection. Milestones A through D are complete for the initial Linux/amd64 scope, with a local [security review](security.md), [platform evidence](platforms.md), and verified [stable public downloads and installation](releases.md#stable-publication). MPL 2.0, GitHub Releases, and public issue reporting remain the policies. Native qualification beyond Linux/amd64 remains future work.

These pages distinguish the implementation from the original design. The [decision register](decisions.md) records adopted choices and remaining questions. The [v0.2 daemon/queue contract](daemon-and-queue.md) and [ordered plan](roadmap.md#v02-implementation-order) are accepted. E1 through E6 are complete for the Linux/amd64 prerelease: private storage, queue controls/concurrency/recovery, shared bandwidth, Linux IPC, the daemon/queue CLI, and release qualification. The commands are available in published `0.2.0-rc.1`; stable `0.1.0` remains standalone and Latest. E6 crash, capacity, concurrency, and shared-rate process fixtures pass the final native ordinary/race/CGO-disabled gates and six-target compilation. The [published 0.2.0-rc.1 prerelease](releases.md#v02-hosted-qualification-and-publication) passed hosted CI, archive checks, anonymous downloads, and installed-binary verification.

The owner removed the TUI from the roadmap before implementation. The next product direction and v0.3 scope await clarification; earlier profile, inspection/history, and hook proposals are not an implementation queue.

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
| [Daemon and queue](daemon-and-queue.md) | Linux daemon/queue commands, private storage/IPC, scheduling, shared budgets, and lifecycle |
| [Security](security.md) | File safety, credentials, network trust, future daemon risks |
| [Development](development.md) | Current checkout, proposed build workflow, coding conventions |
| [Testing](testing.md) | Deterministic tests, fault cases, release evidence |
| [Platforms](platforms.md) | Native Linux evidence, cross-build results, reproduction commands, and limits |
| [Releases](releases.md) | CI, clean-snapshot packaging, draft workflow, and installation |
| [Troubleshooting](troubleshooting.md) | Expected failures and safe recovery guidance |
| [Roadmap](roadmap.md) | Milestones, acceptance criteria, deferred features |
| [Open decisions](decisions.md) | Unsettled interfaces and implementation policies |

## Design source map

The original design is preserved as a historical proposal. Its TUI references are superseded by the owner's removal decision; the current roadmap and decision register govern new work. This map shows where its subjects are explained in detail.

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

`v0.1` is released as `0.1.0`; `v0.2` is available as the `0.2.0-rc.1` prerelease, and the scope of `v0.3` is pending. A specified contract is not an implemented command or runtime proof. “Later” means unscheduled. “Illustrative” means an example is not a stable schema or output contract.

Remote example URLs are placeholders. Build commands use the actual repository layout; choose output paths appropriate to your filesystem. A successful-looking progress display is never evidence that a transfer has passed verification.
