# Configuration

[Documentation index](README.md)

The current CLI uses built-in defaults and flags. No configuration loader exists yet; the TOML format below remains a proposal.

## Current defaults

The engine uses up to four workers for fresh transfers with safe range support, automatic compatible sequential resume, no retries, no overwrite, and no rate cap. `--connections`/`-c` accepts 1 through 32; one selects a direct stream. The network inactivity timeout is `30s`. See [CLI](cli.md) for all flags. Explicit `--resume` requires saved state; parallel partial resume is not implemented yet.

No configuration directories or files are created or read. Transfer artifacts are the partial data file, versioned JSON manifest, and persistent lock file beside the output. Temporary manifest names can remain after a crash and are never trusted for recovery.

## Planned v0.1 defaults

| Setting | Proposed value | Meaning |
|---|---|---|
| Connections | `4` | Desired range-worker count, reduced when necessary |
| Retries | `3` per range | Bound transient retry behavior; exact attempt counting remains open |
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

Do not create or depend on the reserved directories until a feature needs them. The design's Unix-style path is not a settled Windows/macOS discovery policy; platform paths and XDG handling remain [open](decisions.md#storage-and-recovery-decisions).

Transfer sidecars are stored beside the destination, outside this configuration root:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

That exception makes interrupted CLI transfers discoverable without a central service. Deleting the configuration folder must not be described as deleting every transfer artifact.

## Units

The size/rate parser should distinguish decimal and binary byte units explicitly:

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

The unit table explains intended semantics, not an implemented grammar. Accepted units, fractional values, whitespace, case sensitivity, overflow, and treatment of zero still need a parser contract. Arbitrary size expressions are outside the v0.1 requirement.

## Validation to define

Invalid CLI values are rejected before network or file work. Worker counts already accept only decimal integers from 1 through 32; the future configuration loader must preserve that contract. Retry counts need an explicit relationship to total attempts. Speed values still need bounded, unambiguous parsing.

Configuration errors should name the offending setting without exposing credentials or URL tokens. Unknown-key handling and the meaning of `resume = false` alongside existing sidecars remain open; it must never imply silent destruction of partial data.

See the [CLI reference](cli.md), [concurrency model](concurrency.md), and [open decisions](decisions.md) for related behavior.
