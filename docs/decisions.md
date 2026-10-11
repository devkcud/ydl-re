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
- Keep version-1 prefix reads, writes, and sequential resume unchanged, with no implicit migration. Resume version-2 partials only after local chunk checks and a conditional one-byte GET validate the saved identity. Keep saved chunk boundaries and re-fetch whole missing chunks. Empty range state is retained, and unacknowledged gaps never count as completed work.
- Prove fresh range capability with a one-byte ranged GET and clean body completion. Select original/effective URL fingerprints, length, and strong ETag by value; later responses must match that identity and their exact assigned bounds.
- Before accepting any saved/downloaded bytes, reuse an ignored-range `200` as an untouched sequential stream, or make one full GET after a valid but ineligible `206` or a `416`. Discard probe bytes and adopt the full response's own metadata. Malformed probes fail; this adds no retry loop and does not change saved-state refusal. See [fresh probing](http.md#fresh-capability-probing).
- Use four workers by default, with a maximum of 32 and a reduction to the number of missing chunks. `--connections`/`-c` accepts decimal digits for 1 through 32, including leading zeros; signs, zero, fractions, hexadecimal, and overflow are refused. One connection streams fresh files directly or resumes saved range tasks with one worker. Saved version-1 state always resumes sequentially.
- A fully covered `partial` range checkpoint still needs remote validation before publication. Empty range checkpoints use a full conditional GET with `If-Match`, exact zero length, and clean EOF. A refused/canceled validation never mutates saved artifacts or falls back to a fresh stream; verified `ready` recovery remains offline.
- Bound the queue to 256 intervals and allocate one 64 KiB payload buffer per worker. Storage grants exclusive incomplete-chunk writers, serializes commits, and blocks file closure/publication while claims exist. The coordinator serializes progress, cancels peers on terminal failure, joins all workers, and verifies exact coverage before publication. Retries keep their worker slot and immutable range assignment; active range state never falls back destructively.
- The implemented retry policy uses three additional attempts by default, with a ceiling of ten and zero disabling retries. A range shares one budget across request and body failures. Retry only `408`, `429`, `500`, `502`, `503`, `504`, and selected transient network failures; identity/protocol/storage failures are terminal.
- Backoff uses equal jitter, starting at 125–250 ms and capped at eight seconds. Valid `Retry-After` values can extend a wait to 30 seconds; longer hints stop instead of retrying early. Malformed/duplicate hints use normal backoff. Waits honor cancellation.
- `--retries` accepts decimal digits from 0 through 10, allowing leading zeros and rejecting signs/fractions/overflow. Each probe or saved-range validation has one request/body budget, each chunk has one budget, and sequential/fallback request setup has its own budget. Sequential body interruption still requires later resume; `416` diagnostic GETs are never retried. The library uses nil for the default and an explicit zero pointer to disable retries.
- Before a chunk's replacement attempt, remove the failed attempt's received bytes from useful progress. Progress may decrease; retries do not inflate total bytes or average useful-byte speed. Storage still commits only successfully verified whole chunks.

## Milestone D decisions made

- `--limit` accepts a positive ASCII integer followed by `B/s`, `KB/s`, `MB/s`, `GB/s`, `KiB/s`, `MiB/s`, or `GiB/s`. Leading zeros are accepted; signs, fractions, whitespace, different casing, zero, and values exceeding signed 64-bit bytes per second are refused. Omission means unlimited. `Request.Limit` uses bytes per second with zero for unlimited; negative library values are refused before file/network work.
- Use one body-byte budget per standalone transfer, including probes, redirects read by the HTTP client, and failed attempts. Initial/idle credit is `min(65536, ceil(bytes_per_second / 10))` bytes. Reservations and cancellation do not refund received bytes; saved file bytes consume none. Timers round up to nanoseconds; controlled-clock rounding tolerance is one microsecond and HTTP sampling tolerance is five milliseconds. Socket read-ahead and protocol overhead are outside this application-level cap. Throttle waits are cancellable and excluded from network inactivity timeouts.

## Command and output decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| C05 | JSON event/result schema, framing, stream routing, `--quiet` interaction, compatibility | Makes machine output safe to script against | [CLI](cli.md) |
| C08 | Future event schema, retry counters, and subscriber backpressure | Extends the implemented CLI snapshots for other consumers | [Concurrency](concurrency.md) |
| C09 | TOML loader, precedence, and unknown keys | Turns the illustrative configuration into a contract while retaining implemented value validation | [Configuration](configuration.md) |

The default worker preference is four, the retry default is three additional attempts, and the rate is unlimited unless a cap is configured.

## Storage and recovery decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| S03 | Destination normalization, name sanitization, parent creation, symlinks, artifact collisions | Defines safe path ownership across platforms | [Security](security.md) |
| S06 | Windows publication and wider durability qualification | Extends the implemented Linux publication and manifest-reconciliation policy safely | [Lifecycle](transfer-lifecycle.md#7-finalize) |
| S08 | Platform config discovery and XDG handling | The suggested Unix path does not define a portable loader | [Configuration](configuration.md) |

Version 1 uses a half-open sequential prefix. Version 2 uses explicit whole-chunk intervals with independent digests and a fixed chunk size. Neither accepts the original illustrative inclusive range-array format. See the [storage contracts](resume-and-storage.md).

Atomic final publication is implemented and tested locally on Linux. Windows publication, native macOS qualification, and deeper filesystem crash-durability evidence remain open.

## HTTP and scheduling decisions

The implemented rate grammar and HTTP budget follow the [aggregate accounting contract](concurrency.md#aggregate-rate-cap).

The safe fallback is a sequential transfer when parallel ranges cannot be trusted. Applying that fallback to existing partial data still requires an explicit, non-destructive state transition.

## Release and compatibility decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| R01 | License, distribution channels, artifact naming, version metadata, changelog process | The project is intended to be open source but has no license or release tooling yet | [Development](development.md) |
| R02 | Supported OS/architecture/filesystem matrix and native CI coverage | Build success alone does not prove locking, signals, or atomic publication | [Testing](testing.md) |
| R03 | Future daemon-wide bandwidth allocation | The v0.1 cap applies to one standalone transfer; separate processes do not share it | [Roadmap](roadmap.md) |
| R04 | Local IPC, daemon permissions, persistence, and queue scheduling | Required before v0.2, not a blocker for standalone transfers | [Architecture](architecture.md#future-front-ends) |
| R05 | Security contact and disclosure process | Allows real reports without publishing an invented contact | [Security](security.md) |

For R03, the aggregate cap for one transfer is implemented for v0.1. Daemon-wide budget semantics remain uncommitted.

## Already established direction

The proposal already chooses Go, a standalone HTTP(S) CLI first, no mandatory database or daemon, bounded concurrency, conservative resume, explicit completed-file overwrite, no automatic execution, and verification before final publication.

Completing the decisions above should make those guarantees concrete. It should not silently add deferred protocols, plugins, dashboards, or workflow engines to the first release.
