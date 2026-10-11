# Open decisions

[Documentation index](README.md)

The original proposal leaves several implementation contracts unsettled. These are explicit gaps, not implemented behaviors. Recommendations in the topic guides remain recommendations until adopted with code and tests.

Resolve each decision before the milestone or interface depending on it is declared complete. Update the owning page when a choice is made; preserve the design's no-corruption and no-silent-overwrite requirements.

## Milestone A decisions made

- Use Go 1.27 or newer and the local module path `yoinkdl`. No remote/module publishing identity is assumed.
- Use the standard library, including `flag` for the CLI; no external dependencies are required.
- Require an existing output parent, reject destination symlinks/special files and reserved sidecar suffixes, and preserve incompatible artifacts.
- Use exclusive partial-file creation, sync before publication, hard-link publication without overwrite, and rename with explicit overwrite. Linux is the initially tested platform; macOS still needs native validation and Windows publication is deferred.
- Sequential transfers request identity encoding and save served bytes unchanged, including any encoding returned anyway. Known lengths refer to those same bytes.
- Use a 30-second network inactivity timeout for connection, TLS, response headers, and individual body reads, with no total transfer deadline. Reject nonpositive CLI timeout values.
- Allow at most ten requests in a redirect chain, reject HTTPS downgrades and URL credentials, and strip sensitive headers across origins.
- Use exit codes `0` for success/help, `1` for transfer/output failures, `2` for command/option errors, and `130` for interruption. Success prints an absolute path to stdout; progress/errors go to stderr.
- Render progress at most every 200 ms while data arrives, plus a final snapshot; use average rate since start. Build versions default to `dev` and can be set through the Makefile.

## Milestone B decisions made

- Manifest version 1 stores a sequential prefix count: `completed = n` means `[0, n)`. Store a SHA-256 digest of that prefix; reject incomplete, duplicate, unknown, or invalid fields.
- Store only SHA-256 identities for the original and effective URLs, with fragments removed. Require exact matching identities; refreshed signed URLs do not silently reuse state.
- Keep a persistent `.yoinkdl.lock` inode and use a nonblocking OS advisory lock. Process death releases ownership; never unlink a lock to guess whether an owner is stale.
- Sync bytes before publishing checkpoints, after four MiB or one second of arriving data and on normal interruption. Only verified response completion produces a `ready` full-file checkpoint.
- Require a known length, identity encoding, and a usable strong ETag to reuse partial bytes. Last-Modified, weak tags, and missing validators do not authorize reuse.
- Probe through actual GET responses. A resume response must match the requested range, total, ETag, and effective URL identity. Refuse mismatches and ignored ranges without destructive fallback.
- Default downloads may resume compatible saved state automatically. Explicit `--resume` requires existing metadata; it never creates a fresh transfer. `--overwrite` remains permission to replace a completed destination, not incompatible state.
- Recover an interrupted publication locally only from a `ready` checkpoint whose complete bytes still match its checksum. Completed metadata is removed after safe publication; the lock file remains.
- `inspect` reads local manifest metadata without network access, hashing data, acquiring writer ownership, or changing artifacts. Missing/corrupt metadata exits `1`; malformed command arguments exit `2`. It does not provide completed-download history.

## Milestone C decisions made

- Use half-open whole chunks with a default size of at least four MiB, enlarged to keep at most 256 chunks per representation. Persist that size with range state so resumed plans keep the same boundaries. Completed chunks must be sorted and disjoint; adjacent chunks remain separate. Zero-length objects need no tasks.
- Use schema version 2 for range state: persist `chunk_size` and sorted `completed_ranges` with individual SHA-256 digests. Sync and validate each whole chunk before saving it. Derive saved-byte totals from coverage; a ready snapshot additionally requires a full-file digest.
- Keep version-1 prefix reads, writes, and sequential resume unchanged, with no implicit migration. Refuse version-2 partial downloads until the parallel HTTP path exists; allow inspection and verified ready recovery for both formats. Empty range state is retained, and unacknowledged gaps never count as completed work.
- Prove fresh range capability with a one-byte ranged GET and clean body completion. Select original/effective URL fingerprints, length, and strong ETag by value; later responses must match that identity and their exact assigned bounds.
- Before accepting any saved/downloaded bytes, reuse an ignored-range `200` as an untouched sequential stream, or make one full GET after a valid but ineligible `206` or a `416`. Discard probe bytes and adopt the full response's own metadata. Malformed probes fail; this adds no retry loop and does not change saved-state refusal. See [fresh probing](http.md#fresh-capability-probing).

## Command and output decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| C02 | Bounds and grammar for future connections, retries, and rate options | Keeps new options unambiguous; current timeout syntax/defaults are implemented | [CLI](cli.md) |
| C03 | Whether `--retries 3` means three retries after the first attempt; budgets for probes and streams | Defines bounded attempts consistently across modes | [Concurrency](concurrency.md) |
| C05 | JSON event/result schema, framing, stream routing, `--quiet` interaction, compatibility | Makes machine output safe to script against | [CLI](cli.md) |
| C08 | Future event schema, retry counters, and subscriber backpressure | Extends the implemented CLI snapshots for other consumers | [Concurrency](concurrency.md) |
| C09 | TOML loader, precedence, unknown keys, rate/unit grammar, overflow/zero handling | Turns the illustrative configuration into a contract | [Configuration](configuration.md) |

The default worker preference is four, the proposed retry default is three per range, and the default rate is unlimited. Preserve those choices unless a deliberate product change is made. The exact parser and boundary behavior still need decisions.

## Storage and recovery decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| S03 | Destination normalization, name sanitization, parent creation, symlinks, artifact collisions | Defines safe path ownership across platforms | [Security](security.md) |
| S06 | Windows publication and wider durability qualification | Extends the implemented Linux publication and manifest-reconciliation policy safely | [Lifecycle](transfer-lifecycle.md#7-finalize) |
| S08 | Platform config discovery and XDG handling | The suggested Unix path does not define a portable loader | [Configuration](configuration.md) |

Version 1 uses a half-open sequential prefix. Version 2 uses explicit whole-chunk intervals with independent digests and a fixed chunk size. Neither accepts the original illustrative inclusive range-array format. See the [storage contracts](resume-and-storage.md).

Atomic final publication is implemented and tested locally on Linux. Windows publication, native macOS qualification, and deeper filesystem crash-durability evidence remain open.

## HTTP and scheduling decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| H05 | Worker ceiling, queue and buffer bounds | Chunk size and plan limits are implemented; runtime concurrency remains to be bounded | [Concurrency](concurrency.md) |
| H06 | Retry allowlist, backoff/jitter bounds, `Retry-After` ceiling | Avoids infinite waits and inappropriate repeated requests | [Concurrency](concurrency.md#retries) |
| H07 | Limiter burst size, timing tolerance, and probe/retry accounting | Gives `--limit` a measurable testable meaning | [Concurrency](concurrency.md#aggregate-rate-cap) |

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
