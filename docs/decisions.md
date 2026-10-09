# Open decisions

[Documentation index](README.md)

The original proposal leaves several implementation contracts unsettled. These are explicit gaps, not implemented behaviors. Recommendations in the topic guides remain recommendations until adopted with code and tests.

Resolve each decision before the milestone or interface depending on it is declared complete. Update the owning page when a choice is made; preserve the design's no-corruption and no-silent-overwrite requirements.

## Milestone A decisions made

- Use Go 1.27 or newer and the local module path `yoinkdl`. No remote/module publishing identity is assumed.
- Use the standard library, including `flag` for the CLI; no external dependencies are required.
- Require an existing output parent, reject destination symlinks/special files and reserved sidecar suffixes, and preserve incompatible artifacts.
- Use exclusive partial-file creation, sync before publication, hard-link publication without overwrite, and rename with explicit overwrite. Linux is the initially tested platform; macOS still needs native validation and Windows publication is deferred.

## Command and output decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| C02 | Accepted flag values, bounds, and timeout syntax/defaults | Users need unambiguous validation and predictable long-transfer behavior | [CLI](cli.md) |
| C03 | Whether `--retries 3` means three retries after the first attempt; budgets for probes and streams | Defines bounded attempts consistently across modes | [Concurrency](concurrency.md) |
| C04 | Explicit `--resume` without state; `resume = false`; combination with `--overwrite` | Avoids accidental deletion or ambiguous restart behavior | [CLI](cli.md) |
| C05 | JSON event/result schema, framing, stream routing, `--quiet` interaction, compatibility | Makes machine output safe to script against | [CLI](cli.md) |
| C06 | Interrupt exit status, dedicated error codes, exact final-result semantics | Makes process outcomes predictable | [CLI](cli.md) |
| C07 | `inspect` output and behavior for absent/corrupt state; version output | Prevents invented history or metadata guarantees | [CLI](cli.md) |
| C08 | Progress cadence, speed smoothing, retry counters, subscriber backpressure | Keeps UI observations consistent without controlling correctness | [Concurrency](concurrency.md) |
| C09 | TOML loader, precedence, unknown keys, rate/unit grammar, overflow/zero handling | Turns the illustrative configuration into a contract | [Configuration](configuration.md) |

The default worker preference is four, the proposed retry default is three per range, and the default rate is unlimited. Preserve those choices unless a deliberate product change is made. The exact parser and boundary behavior still need decisions.

## Storage and recovery decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| S01 | Manifest fields, nullability, enums, unknown sizes, validation, migration policy | Recovery cannot depend on an illustrative JSON shape | [Resume and storage](resume-and-storage.md) |
| S02 | Inclusive or half-open persisted intervals | Prevents off-by-one corruption; the original sample is inclusive | [Resume and storage](resume-and-storage.md#interval-convention) |
| S03 | Destination normalization, name sanitization, parent creation, symlinks, artifact collisions | Defines safe path ownership across platforms | [Security](security.md) |
| S04 | Exclusive lock primitive and stale-lock recovery on each OS | Prevents simultaneous writers without breaking active ownership | [Resume and storage](resume-and-storage.md) |
| S05 | Checkpoint interval, durability ordering, conservative re-fetch, temporary files | Ensures saved metadata never overstates recoverable bytes | [Resume and storage](resume-and-storage.md#checkpoint-ordering) |
| S06 | Atomic publication, overwrite races, directory durability, cleanup-failure outcome | Makes completion and recovery trustworthy on each supported platform | [Lifecycle](transfer-lifecycle.md#7-finalize) |
| S07 | Refusal/restart interaction for changed resources or incompatible artifacts | Avoids silently discarding data or combining versions | [Resume and storage](resume-and-storage.md) |
| S08 | Platform config discovery and XDG handling | The suggested Unix path does not define a portable loader | [Configuration](configuration.md) |
| S09 | Credential-bearing URL persistence, redaction, and refreshed signed-URL matching | Preserves identity without unnecessary secret exposure | [Security](security.md) |

Internal half-open intervals are the design's preferred arithmetic convention. Changing the sample to that convention on disk is still a schema decision, not a documentation-only substitution.

Atomic final publication is a product requirement. Its platform implementation is unresolved; writing “rename” in a design does not prove that every operating system meets it.

## HTTP and scheduling decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| H01 | Probe order and behavior for empty resources or unreliable metadata | Selects a safe mode without relying solely on `HEAD` or headers | [HTTP](http.md) |
| H02 | Validator selection, strong-date eligibility, missing/weak-validator policy | Protects both resume and parallel transfers against version mixing | [HTTP](http.md#representation-validators) |
| H03 | Exact fallback/restart transition after `200`, `416`, or a changed representation | Prevents concurrent old/new plans writing incompatible bytes | [HTTP](http.md) |
| H04 | Sequential content-encoding policy | Determines which bytes are saved and which length can be verified | [HTTP](http.md#encoding-and-unknown-lengths) |
| H05 | Minimum chunk size, worker ceiling, queue and buffer bounds | Keeps concurrency and memory bounded without tiny-range overhead | [Concurrency](concurrency.md) |
| H06 | Retry allowlist, backoff/jitter bounds, `Retry-After` ceiling | Avoids infinite waits and inappropriate repeated requests | [Concurrency](concurrency.md#retries) |
| H07 | Limiter burst size, timing tolerance, and probe/retry accounting | Gives `--limit` a measurable testable meaning | [Concurrency](concurrency.md#aggregate-rate-cap) |
| H08 | Redirect limit and credential forwarding/downgrade policy | Prevents secret leakage and ambiguous representation changes | [Security](security.md) |

The safe fallback is a sequential transfer when parallel ranges cannot be trusted. Applying that fallback to existing partial data still requires an explicit, non-destructive state transition.

## Release and compatibility decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| R01 | License, distribution channels, artifact naming, version metadata, changelog process | The project is intended to be open source but has no license or release tooling yet | [Development](development.md) |
| R02 | Supported OS/architecture/filesystem matrix and native CI coverage | Build success alone does not prove locking, signals, or atomic publication | [Testing](testing.md) |
| R03 | Final v0.1/v0.2 bandwidth boundary | The detailed draft puts `--limit` in v0.1 and also mentions bandwidth limits in v0.2 | [Roadmap](roadmap.md) |
| R04 | Local IPC, daemon permissions, persistence, and queue scheduling | Required before v0.2, not a blocker for standalone transfers | [Architecture](architecture.md#future-front-ends) |
| R05 | Security contact and disclosure process | Allows real reports without publishing an invented contact | [Security](security.md) |

For R03, these docs follow the concrete CLI and hardening milestone by treating an aggregate cap for one transfer as planned v0.1 behavior. Daemon-wide budget semantics remain uncommitted.

## Already established direction

The proposal already chooses Go, a standalone HTTP(S) CLI first, no mandatory database or daemon, bounded concurrency, conservative resume, explicit completed-file overwrite, no automatic execution, and verification before final publication.

Completing the decisions above should make those guarantees concrete. It should not silently add deferred protocols, plugins, dashboards, or workflow engines to the first release.
