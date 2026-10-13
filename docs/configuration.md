# Configuration

[Documentation index](README.md)

The current CLI uses built-in defaults and flags. No configuration loader exists yet; the TOML format below remains a proposal.

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

## Proposed configuration file

The design suggests `~/.config/yoinkdl/config.toml`:

```toml
[download]
connections = 4
retries = 3
resume = true

[network]
limit = "5MB/s"
```

This example enables a cap. Omit `network.limit` for the proposed unlimited default. The configuration schema is illustrative; supported keys, strictness, and parser dependency have not been selected.

Recommended precedence is built-in defaults, then the optional file, then explicit CLI flags. This ordering still needs to be adopted and tested. No environment-variable overrides or custom config-path flag are specified.

## Paths

The proposed organization is:

```text
~/.config/yoinkdl/
├── config.toml
├── state/
├── logs/
└── profiles/
```

| Path | Intended use | Availability |
|---|---|---|
| `config.toml` | Optional user settings | Proposed |
| `state/` | Daemon and persistent queue state | Reserved for later |
| `logs/` | Persistent logs | Reserved for later |
| `profiles/` | Named configuration profiles | Reserved for later |

Do not create or depend on the reserved directories until a feature needs them. This original layout remains illustrative. The [v0.2 daemon contract](daemon-and-queue.md#paths-and-access) instead selects XDG state and runtime roots for Linux; cross-platform configuration-file discovery remains open.

Transfer sidecars are stored beside the destination, outside this configuration root:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

That exception makes interrupted CLI transfers discoverable without a central service. Deleting the configuration folder must not be described as deleting every transfer artifact.

## Planned daemon settings

These settings are accepted for v0.2 but are not implemented. No TOML loader is required. The daemon alone creates its private state/runtime roots; standalone commands retain the current no-config-directory behavior.

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
