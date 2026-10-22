# Open decisions

[Documentation index](README.md)

The original proposal leaves several implementation contracts unsettled. These are explicit gaps, not implemented behaviors. A selected contract can guide implementation; only code and its required validation establish working behavior.

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
- `download --json` emits schema-version-1 newline-delimited `progress` and terminal `result` objects on stdout. Quiet mode keeps only the result. Progress is advisory and coalesced to 200 ms; results distinguish transfer status from publication and retained partial paths. Unknown totals/paths are null. Argument errors/help remain text; failed JSON output uses stderr. See [JSON output](cli.md#json-output) for fields and compatibility.
- JSON writes use bounded-memory synchronous backpressure. Linux/macOS pipe/socket handles are duplicated above the standard descriptors with close-on-exec, cancellable progress writes, five-second write deadlines, and recoverable broken-pipe errors. Failed/short output stops the stream and cancels transfer work; terminal reporting follows worker/storage cleanup. No progress subscriber authorizes publication. Additional subscriptions require a concrete future requirement.
- Text rendering quotes/escapes nonprinting characters without changing filesystem paths or JSON values. Ordinary printable path output stays compatible. Use ANSI refresh only on a detected terminal with a nonempty, non-`dumb` TERM; cap progress below the current reported width. Final progress shows zero active workers.
- Text and JSON diagnostics reuse the bounded writer. Failed/short progress writes cancel transfers; output failure after publication retains the verified file and reports its path when possible. Pipes, sockets, and detected terminals use owned cancellable descriptors with five-second per-write deadlines. Text stdout setup is checked before transfer work; quiet mode does not require unused stderr. Help/version/inspection output failures exit 1, invalid invocations retain 2 even with unavailable diagnostics, and interrupted output exits 130. Never send diagnostics to stdout as a fallback.
- Omit invalid option values and raw HTTP parser failures from diagnostics. Disable transport connection reuse to prevent unsolicited HTTP/1 bytes from reaching the default logger, accepting extra connection setup. Explicit runtime tracing and caller-visible local paths remain outside URL redaction. Every full `200` response must omit `Content-Range`, including zero-prefix resume.
- Current Unix storage requires an existing parent, anchors operations to that opened directory, refuses destination/sidecar symlinks and special files, and uses private creation permissions. Mutable partials must have one hard link and no group/other permission bits; refusal preserves incompatible artifacts. Verified `ready` recovery retains intentional publication links. The caller must trust the directory and avoid external artifact mutation; advisory locks do not exclude arbitrary external writers.
- Sequential finalization re-reads the complete file and matches its size and checksum against the received stream before making a `ready` checkpoint. Failed verification disables later checkpointing during close. This adds a full local read and complements existing chunk/coverage verification.
- URL-derived names replace nonprinting Unicode characters, separators, and reserved punctuation, preserve printable localized text, and fall back for unusable names or names over 200 bytes. Explicit output paths stay exact. Current Unix path behavior is settled; Windows naming and native filesystem qualification remain open. See [security](security.md).
- Separate the six-target compilation matrix from native runtime qualification. Linux/amd64 has ext4 full-gate/race evidence and an uncached CGO-disabled tmpfs run. Linux/arm64 and macOS remain enabled but unqualified natively; Windows storage stays unsupported despite successful builds. Matrix binaries use CGO disabled and remain validation artifacts, not a distribution promise. See [platforms](platforms.md).

## Command and output decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| C08 | Richer web telemetry, subscriptions, and retry counters | W1 safe polling/reconnect is specified; speed/connection/resource metrics and subscriptions remain deferred | [W1 HTTP interface](local-web.md#http-interface), [concurrency](concurrency.md) |
| C11 | Aggregate profile limits and richer job controls | W1 defaults/association are specified; shared profile budgets, reassignment/deletion, priorities/reordering, and job editing require later contracts | [W1 profiles](local-web.md#profiles-and-effective-settings), [profile model](roadmap.md#instance-and-profile-model) |
| C12 | Rules, workflow execution, and trigger/schedule semantics | Defines metadata stages/precedence, versioned runs, loops/subflows, cancellation, retries, deduplication, downtime, and overlap behavior | [Build phases](roadmap.md#local-web-delivery-order) |
| C13 | Later Firefox interception/bridge and hosted-instance transports | Firefox is selected first; define permissions, pairing/authentication, destination selection, supported captures, takeover/recovery, and duplicate/unknown-outcome handling | [Later integration](roadmap.md#later-browser-integration-and-hosted-operation) |

The default worker preference is four, the retry default is three additional attempts, and the rate is unlimited unless a cap is configured.

## Configuration decisions made

C09 and Linux config discovery in S08 are selected in [Configuration](configuration.md): one strict optional XDG `config.json`, sparse global/profile overrides, shared CLI/web transactions, exact profile selectors, content tokens for stale writes/previews, and `config init/list/get/set/unset` with defined JSON/text results. No configuration implementation exists yet. The earlier TOML proposal is superseded; other platform discovery remains deferred.

The app ledger must become version 2 and retain frozen submission data without editable profiles. Config and app locks are per transaction; a separate lock owns the web process lifetime. Managed CLI/web edits serialize; manual editing is supported offline, with explicit advisory-lock limits. Config changes never retarget accepted jobs or change daemon policy. The [implementation order](configuration.md#delivery-and-validation) begins with a pure config model/resolver, not filesystem or HTTP code.

## Storage and recovery decisions

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| S03 | Windows path normalization, reserved names, reparse points, and artifact collisions | Extends the settled Unix path policy to an unsupported storage platform | [Security](security.md) |
| S06 | Windows publication and wider durability qualification | Extends the implemented Linux publication and manifest-reconciliation policy safely | [Lifecycle](transfer-lifecycle.md#7-finalize) |
| S08 | Non-Linux configuration discovery and storage | Linux user config/state/runtime selection is specified; other native platforms remain deferred | [Configuration](configuration.md) |
| S09 | Metadata evolution, library/history, and portable definitions | W1 profile/submission JSON is specified; migration, run/file schemas, retention, source/credential privacy, duplicate identity, and import/export path mapping remain later work | [W1 metadata](local-web.md#private-application-metadata), [application ownership](architecture.md#planned-web-application) |

Version 1 uses a half-open sequential prefix. Version 2 uses explicit whole-chunk intervals with independent digests and a fixed chunk size. Neither accepts the original illustrative inclusive range-array format. See the [storage contracts](resume-and-storage.md).

Atomic final publication is implemented and tested locally on Linux. Windows publication, native macOS qualification, and deeper filesystem crash-durability evidence remain open.

## Managed storage decisions made

S10 is selected in [Managed transfer storage](managed-storage.md): checkpoint roots follow config/profile settings; partials use hidden destination staging or a private root on the same publication filesystem. Output-local persistent locks and bounded locators pin recovery independently of later settings. Existing sidecars resume in place, conflicting/foreign state is preserved, and cross-filesystem copying is deferred.

Managed requests require version-2 queue/IPC fields and immutable storage hashing; validated v1 jobs/clients retain legacy semantics. The updated engine coordinates both layouts. Older standalone/daemon executables cannot safely coordinate on hidden locks and are excluded from concurrent managed-layout use; qualify/document cutover rather than silently deleting old locks. These decisions are not runtime evidence. The [ordered slices](managed-storage.md#implementation-order-and-acceptance) start with C1a and retain independent config, app, storage, queue/IPC, CLI, and frontend changes.

## HTTP and scheduling decisions

The implemented rate grammar and HTTP budget follow the [aggregate accounting contract](concurrency.md#aggregate-rate-cap).

The safe fallback is a sequential transfer when parallel ranges cannot be trusted. Applying that fallback to existing partial data still requires an explicit, non-destructive state transition.

## Release and compatibility decisions

The owner selected MPL 2.0 with no named copyright-holder notice, GitHub Releases for initial Linux/amd64 binaries, and public GitHub Issues for security reports. [LICENSE](../LICENSE) and [SECURITY.md](../SECURITY.md) record these choices. R01/R05 are settled: [the release contract](releases.md) defines version/tag syntax, binary/source archives, checksums, changelog sections, and draft creation. The authorized release work published stable `0.1.0` as Latest and preserved `0.1.0-rc.1` as a separate prerelease. Public downloads, unchanged assets, and installation are verified. Remote repository settings were not changed.

| ID | Decision needed | Why it matters | Owner page |
|---|---|---|---|
| R02 | Native qualification beyond Linux/amd64 | Hosted Linux/amd64 CI passed; cross-compilation does not prove foreign runtime behavior | [Platforms](platforms.md) |

## v0.2 decisions made

R04 is settled by [Daemon and persistent queue](daemon-and-queue.md). Its private queue store, coordinator, Linux IPC runtime, and CLI controls are implemented in the published `0.2.0-rc.1` prerelease. Use a foreground Linux daemon, private pathname Unix IPC with kernel peer-UID checks, distinct state/runtime locks, and an atomic bounded JSON queue. Keep standalone downloads independent and the existing transfer engine authoritative for byte validation/publication.

The owner selected full URL/query-token storage in private local queue files and requested configurable concurrent downloads. The durable `parallel` setting defaults to `1`, accepts `1..8`, and can change live. It governs whole-job admission; per-download worker counts remain separate. Lowering it lets active jobs finish while withholding new work. Crash-interrupted jobs require explicit resume; no-overwrite queued transfers preserve ambiguous publication outcomes. Terminal records can be forgotten without deleting files.

R03 is settled by [daemon-wide budgeting](concurrency.md#daemon-wide-rate-cap): one process-lifetime body budget shared across managed jobs, composed with each job's existing cap. Charge both before waiting, wait for all applicable reservations, preserve shared credit/debt across job lifecycle and concurrency changes, and retain existing burst/timeouts/accounting boundaries. `daemon start --limit` is fixed until restart; cross-process caps and dynamic rate changes are deferred.

Linux XDG state/runtime selection is settled for the daemon; this does not implement a general configuration loader or qualify other native platforms. The [ordered implementation plan](daemon-and-queue.md#delivery-and-validation) and [validation matrix](testing.md#planned-v02-validation) are accepted. E1 through E6 are complete with local Linux tests, final ordinary/race/CGO-disabled gates, six-target compilation, and [local candidate archive/installation verification](releases.md#v02-local-qualification). [Hosted qualification and publication](releases.md#v02-hosted-qualification-and-publication) passed for `0.2.0-rc.1`; stable `0.1.0` remains Latest. The owner authorized pushes/publication for this named prerelease only.

## Web application direction

The owner approved a local-first web application with hosted operation second. The roadmap now tracks feature-building phases rather than version targets or release preparation. The TUI remains canceled; its input/rendering/polling contract is not reused as a requirement for the web app.

Vue 3 + TypeScript is the frontend direction, and **Vue Flow is selected** for the Visual Grabber editor. Go remains the application/daemon integration direction, with an optional Playwright browser worker. The editor choice does not select a workflow execution engine, data schema, or additional runtime dependency for standalone downloads.

An instance identifies an execution host/output filesystem; a profile is a workspace within that instance. Basic local downloads/profiles lead W1, then library/history and rules in W2, schedules and a small browser/workflow runner in W3, and fuller Vue Flow authoring/debugging in W4. The first-slice choices are recorded below; no new application behavior is implemented.

The owner explicitly deferred browser integration and hosted targeting to L1/L2 and selected **Firefox as the first extension target**, including opt-in interception of supported downloads. The intended later actions include **Send this link to Yoink** and **Send download to → Home server → Pato**, where Home server is an instance and Pato is a profile. Chromium support can be considered later; the interception mechanism and bridge remain open. These integrations are not prerequisites for W1. Custom hooks, arbitrary scripting, and distributed transfer swarms are separate unscheduled directions.

### W1 contract decisions made

[W1 local web](local-web.md) resolves C10 and the W1 portions of C08, C11, and S09 as implementation contracts, not runtime evidence:

- Run an explicit separate foreground web process on numeric IPv4 loopback, serving built Vue/TypeScript/Vite assets and a versioned API. Use a per-launch bearer token kept in tab memory, exact host/origin checks, and no CORS/cookies. Web shutdown never stops the daemon; Go-only builds remain independent of Node/assets.
- Reuse existing safe polling and queue controls through `daemon.Call`. Keep nullable outcomes and decimal-string web integers, bound clients/bodies, never retry unknown mutations automatically, and defer speed/ETA/subscriptions, daemon start/stop buttons, and dynamic global caps.
- Store editable profiles only in config.json; keep URL-free immutable submission bindings in the separate version-2 app ledger. Require output-directory setup, freeze settings plus config token, persist before Add, and never replay intents. Unprofiled CLI jobs remain Unassigned; profiled CLI admission must share the binding service. These choices supersede the earlier version-1 app-store plan, not the already tested version-1 pure implementation.
- Keep Linux/amd64 as the first native qualification scope. The [acceptance cases](local-web.md#acceptance-and-failure-cases) and [delivery slices](local-web.md#delivery-order) follow the October 22 [selected configuration foundation](configuration.md#delivery-and-validation) and required ledger revision. W2+ metadata evolution and L1/L2 remain separate.

## Already established direction

The proposal already chooses Go, a standalone HTTP(S) CLI first, no mandatory database or daemon, bounded concurrency, conservative resume, explicit completed-file overwrite, no automatic execution, and verification before final publication.

Completing the decisions above should make those guarantees concrete. It should not silently add deferred protocols, plugins, dashboards, or workflow engines to the first release.
