# Configuration

[Documentation index](README.md)

The current CLI uses built-in defaults and flags. No configuration loader exists yet; the TOML format below remains a proposal.

The [W1 web contract](local-web.md#profiles-and-effective-settings) currently specifies profile defaults in a private `<daemon-state>/web/app.json`, with effective settings frozen at submission and CLI-created jobs Unassigned. Its pure model/codec exists; the filesystem store and configuration loader do not. The new [design checkpoint](#configuration-and-storage-design-checkpoint) must reconcile profile configuration ownership before W1.1b. The initial Default profile still requires directory setup, and profile switching/editing must never change accepted jobs. Shared profile budgets remain a later contract.

## Current defaults

The engine defaults to up to four workers, three additional attempts for transient failures, automatic compatible resume, no overwrite, and no rate cap. `--connections`/`-c` accepts 1 through 32; `--retries` accepts 0 through 10, with zero disabling repeats. `--limit` sets an aggregate body rate using the units below. One connection streams new files directly and resumes saved range tasks with one worker. The network inactivity timeout is `30s` and excludes throttle/backoff waits. See [CLI](cli.md) for all flags. Explicit `--resume` requires saved state.

No configuration directories or files are created or read. Transfer artifacts are the partial data file, versioned JSON manifest, and persistent lock file beside the output. Temporary manifest names can remain after a crash and are never trusted for recovery.

## Transfer defaults

| Setting | Value | Meaning |
|---|---|---|
| Connections | `4` | Desired range-worker count, reduced when necessary |
| Retries | `3` additional attempts | Separate request/probe budgets and one shared request/body budget per range; sequential bodies use later resume |
| Resume | Enabled for compatible partial state | Revalidate before trusting saved bytes |
| Rate limit | Absent | No configured transfer cap |
| Overwrite | Disabled | Preserve a completed destination unless explicitly allowed |
| Timeout | `30s` | Current network inactivity timeout; no total transfer deadline |

The standalone CLI must work with defaults and no configuration file, daemon, or database.

## Configuration and storage design checkpoint

Requested direction, October 22, 2026: provide an optional XDG configuration file, make download settings and state placement follow the selected profile, and keep per-transfer bookkeeping out of the destination's file listing. This is planning work before W1.1b, not a configuration loader or storage-layout change. Additional settings can be added when concrete workflows need them.

Separate these scopes:

| Scope | Responsibility |
|---|---|
| Instance | Configuration discovery, base state/runtime locations, and shared daemon policy; selecting a profile must not select or restart a different daemon |
| Profile | Output directory, transfer defaults, and the proposed download-state/staging location within that instance |
| Accepted job | Captured effective settings and artifact locations; later profile edits or root changes must not redirect recovery or move its files |

Proposed precedence is built-in defaults, user defaults, selected profile, then allowed explicit invocation overrides. Preserve omission versus explicit unlimited values and existing validation. This does not enable new W1 request overrides or overwrite behavior. Define CLI profile selection and association explicitly; do not infer it from output paths.

The configuration file and web editor must share one authority for profile settings. Decide how manual edits, web edits, stable IDs, and revisions interact with the current `app.json` model before persisting it. Do not maintain independently editable TOML and JSON copies of the same defaults. Keep preferences separate from checkpoint progress, submission intents, and other recovery state.

Small recovery metadata belongs under the selected persistent state root, with XDG state as the proposed default. Partial download bytes need a separate staging policy: the current atomic hard-link/rename publication requires the destination filesystem. A private hidden staging directory on that filesystem is a candidate. A configured directory on another filesystem must either be refused explicitly or use a separately qualified copy, sync, verification, and final publication path. It cannot be handled as a simple rename.

Before implementation, settle profile ownership and file format, root discovery and precedence, metadata versus partial-data placement, and compatibility with existing sidecars. Resume and `inspect` must still find old jobs after settings change. Writers targeting the same destination must contend on one stable ownership mechanism even when they use different profiles or state roots. Preserve existing partials/manifests and lock identity; deleting leftover locks is not a cleanup strategy. Keep loader, storage relocation, and web integration in separate focused changes.

## Proposed configuration file

The existing TOML proposal would live at `$XDG_CONFIG_HOME/yoinkdl/config.toml`, falling back to `~/.config/yoinkdl/config.toml`. Format and keys remain to be settled; this example does not yet describe profiles:

```toml
[download]
connections = 4
retries = 3
resume = true

[network]
limit = "5MB/s"
```

This example enables a cap. Omit `network.limit` for the proposed unlimited default. The configuration schema is illustrative; supported keys, strictness, and parser dependency have not been selected.

The profile-aware precedence above still needs to be adopted and tested. Environment-variable setting overrides, system-wide configuration merging, and a custom config-path flag are not specified.

## Paths

The proposed separation follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/):

| Purpose | Base directory | Standard fallback |
|---|---|---|
| User configuration | `$XDG_CONFIG_HOME` | `~/.config` |
| Persistent recovery state | `$XDG_STATE_HOME` | `~/.local/state` |
| Disposable cache | `$XDG_CACHE_HOME` | `~/.cache` |
| Live IPC/runtime objects | `$XDG_RUNTIME_DIR` | No standard fixed fallback |

XDG directory overrides must be absolute. Resume metadata and partial bytes are not disposable cache and must survive logout/restart. The existing [Linux daemon path contract](daemon-and-queue.md#paths-and-access), including its private runtime fallback, remains implemented behavior. Profile subdirectories, configurable roots, config discovery/invalid-path handling, and cross-platform support still need their own contract. Do not create directories until the owning feature needs them.

Transfer sidecars are stored beside the destination, outside this configuration root:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

This current layout makes interrupted CLI transfers discoverable without a central service. After successful publication and cleanup, the partial and manifest are removed; the empty persistent `.yoinkdl.lock` remains to prevent competing lock inodes. An empty JSON manifest is not an expected successful-download artifact. Relocation is proposed above; deleting the configuration folder must not be described as deleting every transfer artifact.

## Daemon settings

The repository build implements these v0.2 daemon/queue settings and CLI commands. Published `0.1.0` remains standalone. No TOML loader is required. The daemon alone creates its private state/runtime roots; standalone commands retain the current no-config-directory behavior.

Queue `parallel` is durable, defaults to `1`, and accepts `1..8`. One means sequential downloads; larger values admit concurrent transfers. It is independent of per-transfer `--connections`. An optional `daemon start --parallel N` overrides the saved setting; omission keeps it. `daemon set --parallel N` changes it live and acknowledges only after persistence. Reducing the limit does not stop current downloads; it gates new admissions until the active count fits.

`daemon start --limit R` is a process-lifetime cap across all its jobs. Omission is unlimited; it is not stored with queue policy and cannot be changed through `daemon set`. Each job's own `--limit` remains durable request input. Shared and per-transfer caps compose according to [daemon-wide budgeting](concurrency.md#daemon-wide-rate-cap), without multiplying the configured rate by the concurrent-job count.

Each queued request stores resolved connection/retry/timeout/per-transfer-limit values and an absolute destination. The daemon never resolves a client's relative destination against its own working directory. Full source URLs, including query tokens, are retained in permission-protected queue files by the owner's selected policy; display responses omit them. See [queue requests and persistence](daemon-and-queue.md#source-urls-and-immutable-requests).

## Units

The rate parser distinguishes decimal and binary byte units explicitly:

| Unit | Bytes |
|---|---|
| `B` | 1 |
| `KB` | 1,000 |
| `MB` | 1,000,000 |
| `GB` | 1,000,000,000 |
| `KiB` | 1,024 |
| `MiB` | 1,048,576 |
| `GiB` | 1,073,741,824 |

For rates, `/s` means per second. Thus `5MB/s` is 5,000,000 bytes per second and `5MiB/s` is 5,242,880 bytes per second. Rates apply across all workers of the transfer.

`--limit` accepts ASCII decimal digits followed immediately by one of these units and `/s`. Values must be positive and convert to at most 9,223,372,036,854,775,807 bytes per second. Leading zeros are allowed. Signs, fractions, exponents, whitespace, missing units, different casing, and overflow are rejected. For example, write `1500KB/s` instead of `1.5MB/s`.

Omitting `--limit` means unlimited; explicit zero is invalid CLI rate syntax. The library's `Request.Limit` is a byte-per-second integer, with zero meaning unlimited and negative values rejected before file/network work. Arbitrary size expressions remain outside v0.1.

## Validation to define

Invalid CLI values are rejected before network or file work. Workers accept decimal integers from 1 through 32 and retries from 0 through 10; the future configuration loader must preserve those contracts and the rate grammar above.

Configuration errors should name the offending setting without exposing credentials or URL tokens. Unknown-key handling and the meaning of `resume = false` alongside existing sidecars remain open; it must never imply silent destruction of partial data.

See the [CLI reference](cli.md), [concurrency model](concurrency.md), and [open decisions](decisions.md) for related behavior.
