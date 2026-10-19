# Daemon and persistent queue

[Documentation index](README.md)

This is the accepted v0.2 contract. E1 through E5 are complete locally: private storage, FIFO controls/concurrency, safe views, composed caps, Linux IPC/lifecycle, and explicit daemon/queue CLI commands. E6 crash, capacity, concurrency, and shared-rate process fixtures pass locally; final platform/release qualification remains pending. The released v0.1 CLI has no daemon, queue store, or queue commands. Initial v0.2 runtime qualification targets Linux/amd64; other targets must reject unsupported daemon operations explicitly.

## Scope and ownership

The internal daemon runtime runs the existing transfer engine through a retained `transfer.Budget.Download` runner. Standalone `download`, `inspect`, and `version` keep their current behavior and never contact the daemon or create its directories.

Add only `internal/queue` for durable jobs and scheduling, and `internal/daemon` for local control and process lifecycle. Keep CLI adapters in `internal/cli`. Reuse pure transfer request validation, filename rules, cancellation, checkpoints, and body-rate accounting; do not copy the downloader or make it import daemon code.

No network listener, database dependency, service installation, automatic daemon startup, profiles, hooks, event subscription service, or TUI is required. `daemon start` runs in the foreground; an external process supervisor can manage it. Background spawning is separate work.

## Queue concurrency

The owner requested a choice between sequential and concurrent downloads. Persist `parallel` in queue state: default `1`, accepted values `1..8`. This is an admission limit on whole transfers, separate from per-transfer `--connections` (`1..32`, default `4`).

- `daemon start --parallel N` optionally replaces the stored setting before work begins. Omission preserves the stored value; a new queue starts at `1`.
- `daemon set --parallel N` changes and durably saves the limit while the daemon runs.
- Increasing it admits waiting jobs in FIFO order. Decreasing it lets existing transfers finish and prevents admission until the active count is below the new limit. It does not silently pause or cancel transfers; pause extras explicitly for an immediate reduction.
- Running, pausing, and canceling jobs occupy slots until `Download` returns and releases writers. At most eight transfers and 256 range workers exist; no goroutine is created for every waiting job.

Settings acknowledgments follow the same durable commit rule as job mutations. `list` reports the configured limit and current active count, including temporary excess after a reduction. There are no priorities, automatic preemption, or automatic whole-job retries. The engine's existing bounded retries apply within each attempt.

## Paths and access

Use [XDG state/runtime locations](https://specifications.freedesktop.org/basedir/latest/): persistent state under `$XDG_STATE_HOME/yoinkdl`, defaulting to `$HOME/.local/state/yoinkdl`; local control under `$XDG_RUNTIME_DIR/yoinkdl`. Ignore relative XDG values. When the runtime value is absent or invalid, use `<state>/run` and report that persistent fallback. A configured absolute runtime base must be owned by the effective user with mode `0700`; unsafe ownership/permissions are refused, not silently replaced.

Create application directories as `0700`, owned by the effective user. Existing application directories must be real directories with that ownership and no group/other permissions; refuse symlinks or unsafe modes without chmod or deletion. Parent directories are caller-trusted. Anchor state operations to an opened `os.Root` and require local locking, atomic rename, and directory sync.

Create regular state files as `0600`. Existing snapshots and locks must be regular, owned by the effective user, private, single-linked, and not symlinks. Refuse incompatible artifacts without rewriting their permissions or contents.

| Artifact | Contract |
|---|---|
| `<state>/queue.json` | Strict versioned queue snapshot; private regular file, owner matches, one hard link |
| `<state>/queue.lock` | Persistent `0600` lock inode with exclusive nonblocking OS ownership |
| `<runtime>/daemon.lock` | Separate persistent `0600` lock for endpoint ownership |
| `<runtime>/control.sock` | Pathname Unix stream socket, `0600`, in the private runtime directory |
| `<state>/.queue.tmp-*` | Exclusive private temporary snapshots; never authoritative on recovery |

Acquire both locks before binding or modifying an existing socket, using nonblocking acquisition and releasing everything on failure. This prevents two runtime roots writing one store or two state roots replacing one endpoint. Lock files remain; their presence does not mean the daemon is running.

Require matching effective user IDs from kernel peer credentials on both client and server. Linux pathname sockets and `SO_PEERCRED` provide the relevant [socket primitives](https://man7.org/linux/man-pages/man7/unix.7.html). Do not trust a UID supplied in JSON. Private-directory ownership is also checked; same-user processes and privileged administrators remain inside the filesystem trust boundary.

A stale socket can be removed only while holding both locks, after verifying socket type, owner, and unchanged inode, and receiving connection-refused from a probe. A responding peer, timeout, permission error, symlink, or unrelated file is preserved and reported. Disable unconditional listener unlink-on-close; cleanup removes only the socket inode this daemon created. Overlong socket paths are errors, never truncated or redirected to a shared temporary path.

E4's path/ownership foundation is implemented in `internal/daemon`. `Resolve` reads the XDG/home environment without IO and returns normalized paths plus `Fallback`; unsupported platforms return `ErrUnsupported`. Ownership creates missing state parents, reuses `queue.Open`, then acquires the separate private runtime lock. Socket probing/binding uses the opened directory through Linux `/proc/self/fd`; a socket `O_PATH` handle anchors its initial permission change. `/proc/self/fd` must be available; there is no unanchored fallback. Peer checks use kernel credentials on both connection ends. The protocol and runtime layers use these ownership checks before dispatch and retain both locks through completed shutdown.

## Source URLs and immutable requests

The owner selected persistence of full source URLs, including query tokens, in private local files. `queue.json` deliberately stores that information so queued/paused jobs survive restarts. It is permission-protected plaintext, not an encrypted credential vault. Transfer manifests remain unchanged and continue storing URL fingerprints only.

Normalize requests using the same HTTP(S), user-info refusal, fragment removal, safe filename, timeout, connection, retry, and rate rules as standalone transfers. The client resolves the destination from its own working directory and sends an absolute clean path; the daemon rejects relative paths. An empty `--output` uses the existing safe source filename. Output parents must exist when the transfer starts.

`add` accepts output, connection, retry, timeout, per-transfer limit, and require-resume options. Queue requests never permit overwrite in the initial v0.2 contract; deliberate replacement remains available through standalone `download`. URL and destination are immutable after acceptance. Refreshing a signed URL is a new job and still cannot bypass incompatible transfer checkpoints.

Never return URLs through status/list responses, diagnostics, or logs. Build explicit safe response records rather than serializing stored jobs. Clear the source URL when a job becomes completed or canceled; failed/paused/interrupted jobs retain it for explicit retry. `forget` removes a retained record, not output or sidecar files. Logical removal does not promise erasure from filesystem history or backups.

## Durable snapshot

The queue schema is separate from transfer manifest versions 1 and 2. A fresh snapshot is:

```json
{"version":1,"parallel":1,"next_order":1,"jobs":[]}
```

All fields are required. Each retained job has these fields:

| Field | Meaning |
|---|---|
| `id` | Client-generated 128-bit random identifier, 32 lowercase hex characters |
| `order` | Unique positive FIFO admission order assigned by the daemon |
| `request` | `url`, `output`, `connections`, `retries`, `timeout_ns`, `limit_bps`, `resume`; resolved values, no overwrite field |
| `input_hash` | SHA-256 of the immutable normalized request; supports add deduplication while the record exists |
| `state` | One of the states below |
| `published` | Last-attempt publication result: boolean, or null when unknown/in progress |
| `error` | Bounded sanitized failure/interruption description, otherwise null |

The request hash uses compact Go JSON encoding of the request fields in the table's order before terminal URL removal. It is an identity check, not authentication or encryption. Validate it against the request while the URL is present; retain it after terminal URL removal. Only completed/canceled records may have a null URL. Completed requires `published: true`; nonterminal records keep their immutable request for retry.

E1 implements this model in `internal/queue`: `NewSnapshot` defaults to sequential admission, `NewJob` uses `transfer.Normalize` and requires an already absolute clean output, and `Request.Transfer` creates owned engine options without overwrite. The codec requires terminal URLs to be null, active/interrupted outcomes to be unknown, and failed/completed outcomes to be known. Failed/interrupted jobs require nonempty errors; other states require null errors. Queued/paused/canceled records may retain a previous known or unknown publication result. Returned validation errors never echo source payloads. Strict decoding also rejects lone UTF-16 surrogate escapes instead of accepting lossy replacement characters.

Use at most 256 retained jobs, 16 KiB per URL, 4096 bytes per output path, 4096 bytes per error, and 8 MiB per encoded snapshot. All bounds apply independently, including JSON escaping. Require valid UTF-8, absolute clean output paths without NUL, unique IDs/orders/output paths, supported versions, exact known fields, and correctly typed non-null values except the documented nullable fields. Reject duplicate keys, trailing JSON, invalid states, overflow, and incompatible combinations. Never reset a corrupt or unsupported queue to empty.

`next_order` increases on add and on a real resume to queued; it must exceed every retained order. Refuse signed-64-bit overflow. Retained records, including completed/canceled jobs, reserve their output paths and count toward capacity. Full queues reject new work before mutation; `forget` frees capacity and reservations. Transfer locks still protect aliases, other daemon instances, and direct CLI writers.

One coordinator owns mutations. Serialize a candidate snapshot to a new private temporary file, sync it, rename it to `queue.json`, sync the directory, then expose the new in-memory state and acknowledge. Live progress never triggers queue persistence. A mutation is accepted only after that sequence succeeds.

Before-rename failure preserves the committed state. Rename-success/directory-sync failure has an uncertain durable outcome: stop dispatch, cancel/join active transfers, refuse further mutations, and stop the daemon. Do not write an older in-memory snapshot over the possibly committed file. Restart validates the canonical snapshot; it never promotes temporary files. A missing snapshot with leftover queue temporary files is refused rather than treated as a new empty queue. Preserve unrecognized/corrupt files for explicit recovery.

The implemented `queue.Open` accepts an explicit absolute clean directory with an existing parent, creates the final directory if needed, acquires `queue.lock`, and durably initializes or strictly loads `queue.json`. `Store.Snapshot` returns a deep copy of the last committed state; `Store.Save` owns its candidate, enforces immutable retained requests and monotonic admission orders, and follows the sync sequence above. Before replacement, it rechecks lock identity, file safety, and the canonical snapshot's inode and content digest. These checks operate within the same-user/trusted-parent boundary; callers must honor the advisory lock.

`ErrLocked` means another store holds ownership. `ErrChanged` disables the store after detected artifact changes; `ErrUncertain` does so after uncertain replacement/durability. Both prevent further snapshots and mutations until close/reopen, and ownership remains held until `Close`. Close releases descriptors without unlinking the lock. Only the store's own unchanged temporary inode may be cleaned up; stale or replaced artifacts are preserved. Non-Linux storage returns `ErrUnsupported`. Loading the store alone does not change a saved running job or start a transfer; coordinator preparation owns reconciliation.

## States and operations

| State | Meaning and permitted controls |
|---|---|
| `queued` | Waiting for a slot; pause or cancel directly; repeated resume is a no-op |
| `running` | Attempt intent was durably committed before calling `Download`; pause/cancel first persist their intent |
| `pausing` | Cancellation requested for pause/shutdown; still occupies its slot |
| `paused` | No active attempt; explicit resume places it at the FIFO tail |
| `canceling` | Cancellation requested; still occupies its slot |
| `canceled` | No future attempt; source cleared; cannot resume |
| `interrupted` | Previous daemon died with an unrecorded attempt outcome; explicit resume only |
| `failed` | Engine returned an error after cleanup; explicit resume only |
| `completed` | Engine returned success with verified publication; source cleared; cannot resume |

`resume` accepts paused/interrupted/failed jobs; queued/running is idempotent, while transitions still draining must finish first. `cancel` accepts any non-completed record and is idempotent; a paused/failed/interrupted job can become canceled without an engine call. `forget` accepts any record without an active slot and preserves all transfer files; absent IDs are an idempotent success. Other unknown IDs return not-found.

A scheduling decision first persists `running` with an unknown new-attempt result, then calls the engine. Pause/cancel acknowledgment confirms durable intent, not that workers have already exited. Only the final engine result resolves a live attempt: success plus `Published` means completed even if pause/cancel arrived too late; an error with published output means failed with `published: true`; cancellation without publication settles the requested paused/canceled state. No result deletes verified output.

On daemon restart, queued jobs remain eligible, running becomes interrupted, pausing becomes paused, and canceling becomes canceled. Recovered active states get `published: null`; persist these changes before dispatch and clear canceled URLs. Do not automatically replay interrupted attempts. A crash can occur after file publication but before the queue commit, so file presence/length never proves completion. Explicit retry still uses the engine's checkpoint checks and no-overwrite policy; a pre-existing final file without recoverable metadata is preserved and refused.

`internal/queue/lifecycle.go` implements the pure recovery, pause/resume/cancel, and final-result rules. Resume alone consumes a fresh FIFO order when it actually changes state. Engine failures map to fixed messages for cancellation, timeout, file access, missing publication, published-with-error, or generic failure; arbitrary error text never reaches saved diagnostics. Public error projection also replaces unrecognized stored messages.

`NewCoordinator(store, runner, parallel)` prepares recovery and an optional startup override durably without dispatching; zero preserves the saved setting and a nil runner selects `transfer.Download`. Call `Run(ctx)` once after startup prerequisites are ready. The caller owns the store, must give the coordinator exclusive mutation access, and closes it only after Run returns. Runners must return after all transfer work and progress callbacks finish. `List` returns owned safe views; `Stop` acknowledges durable stopping intent while `Done`/Run signal completed cleanup.

The internal coordinator also implements `Add`, `Pause`, `Resume`, `Cancel`, `Forget`, and `SetParallel`. Add takes a client ID and transfer options with an absolute output; it returns the retained view for an identical ID/hash, including terminal records. Capacity, conflicts, invalid input, and exhausted order counters reject before mutation. Encoded-size refusal is also checked before disk IO and leaves the coordinator available. Forget returns whether a record was removed and never deletes transfer files. While stopping, only reads and repeated stop remain available. These Go APIs are not CLI commands or a listening daemon.

The coordinator owns at most eight attempts, a 16-entry control mailbox, and one latest progress value per attempt. Progress callbacks never wait on disk or control replies. Caller cancellation after enqueue reports `ErrOutcomeUnknown` for mutations and does not undo the queued operation. Abandoned replies cannot block the coordinator. Every persistence failure stops admission, cancels and joins runners, and prevents further saves or authoritative views. If durable pause intent cannot be written, the previous running record remains for conservative restart recovery. Persistence errors preserve their causes for internal classification while exposing a fixed safe message.

## Local control protocol

Use protocol version 1 with one newline-terminated UTF-8 JSON request and response per connection; each frame contains exactly one object. Close after the response; do not execute pipelined requests. Limit request frames to 64 KiB, replies to 8 MiB, concurrent control connections to 16, and the coordinator's control mailbox to 16. Apply five-second socket read/write deadlines. Filesystem sync may outlast a client deadline; it is not canceled by pretending a commit rolled back.

Requests have exactly `version`, `state_dir`, `op`, and `args`. The server compares the requested normalized absolute state directory with its selected directory and refuses mismatches; it never chooses storage from request input. Supported operations are:

| Operation | `args` |
|---|---|
| `ping`, `list`, `stop` | Empty object |
| `add` | `id` and the normalized `request` object |
| `pause`, `resume`, `cancel`, `forget` | `id` |
| `set_parallel` | `parallel` in `1..8` |

Replies have exactly `version`, `ok`, `result`, and `error`. A success has a non-null safe result and null error; a failure has null result and an error containing fixed `code` and sanitized `message`. Refuse unsupported versions, unknown/duplicate fields, null required values, malformed/trailing JSON, and incorrect argument shapes. Do not reflect raw request bodies or URL values.

`internal/daemon/protocol.go`, `json.go`, and `replies.go` implement strict request/reply validation and bounded newline framing. Nested request fields reuse `queue.Request.Transfer` for normalized engine validation. Reply validation checks operation-specific safe views, bounds, and fixed failure messages; raw parser, source, and runner errors are never serialized. Lone Unicode surrogates are refused. Reading one frame never executes another pipelined request.

Job replies expose only `id`, `order`, `state`, `output`, `published`, `error`, and advisory `bytes`, `total`, `workers`; unobserved counters are null. Daemon views expose `state` (running/stopping), `parallel`, `active`, and `limit_bps` (zero means unlimited). List returns a daemon view and jobs ordered by `order`; add/pause/resume/cancel return a job view, `ping`/`stop`/`set_parallel` return a daemon view, and forget returns `id` plus `forgotten`. Stored requests and their hashes never appear in these views.

`add` with an existing ID and identical input hash returns its current safe view; a changed request conflicts. That guarantee ends after explicit forget. Clients do not blindly retry mutations after a timeout/disconnect: report uncertain outcome and the job ID where known, then let the user inspect `list`. Reads may retry. Disconnecting or failing to print an acknowledgment does not undo a committed operation or cancel a queued job.

`list` is polling, not a subscription. It reports queue policy and safe job fields, plus advisory live snapshots or saved-byte metadata when available. Raw sources are absent. Progress may decrease on retry/recovery and never authorizes publication. Slow or broken control clients must not stall the transfer coordinator; bound/coalesce snapshots and isolate response writes. Existing standalone JSON/output-failure semantics remain unchanged.

`daemon.Call(ctx, paths, op, args)` is the internal Linux client. It validates the request locally, checks private runtime/socket ownership and the server's kernel UID, sends once, and validates the operation-specific reply. Empty operations require an empty object for `args`. It creates no directories or processes. A failed write, lost reply, or malformed reply after a mutation is attempted reports an unknown outcome; reads do not gain that mutation warning. Context cancellation interrupts client IO without undoing accepted work.

## Startup and shutdown

`daemon start` creates/validates private roots, acquires both locks, loads/reconciles state, applies an optional parallel override durably, binds/checks the socket, and only then admits work. Do not start implicitly from add/list or standalone commands. Startup failure never deletes or resets queue data and releases ownership. Any reconciliation/setting already committed before a later bind failure remains durable; do not roll it back with an older snapshot.

`daemon stop` durably pauses active attempts, stops admission, cancels their contexts, and returns an accepted-stopping response. It does not claim the process has exited. While stopping, allow reads and repeated stop requests but reject other mutations. Queued jobs remain queued for the next explicit start. Shutdown settles accepted operations, joins all transfers, and persists actual results before removing its socket and releasing locks; no fixed deadline authorizes destructive cleanup. SIGINT/SIGTERM follows the same safe cleanup. Clean IPC stop exits 0, signal interruption 130, and startup/persistence failure 1.

The internal API is `daemon.New(Options{Paths, Parallel, Limit})`, then `Server.Run(ctx)` once. New acquires ownership, durably reconciles the coordinator and optional concurrency override, and binds the endpoint without starting downloads. Run admits work only after that preparation. Close releases an unstarted instance or requests joined shutdown independently of the control mailbox; Done closes after transfer results, client handlers, and ownership cleanup finish. The CLI maps parent-context interruption to exit 130 after cleanup. Non-Linux construction and client operations return `ErrUnsupported`.

At most sixteen accepted client handlers run concurrently. Socket read/write deadlines and bounded coordinator waits are independent of job contexts. Disconnecting an acknowledged or unacknowledged mutation never cancels the job. Stop replies can finish before process cleanup; stalled response writers cannot block coordinator progress, and their deadlines bound handler cleanup. Persistence failures and detected runtime ownership changes stop admission and join transfers; foreign replacement artifacts remain preserved. CLI signals, forced daemon death, pending controls, and publication ambiguity have local [process evidence](testing.md#current-evidence).

## Shared bandwidth

The internal integration uses `transfer.NewBudget(rate)` once, then passes `budget.Download` to `NewCoordinator`. The coordinator retains the runner across controls and attempts; each attempt gets its own private cap while the shared budget survives pause, cancel, failure, completion, resume, and concurrency changes. Controlled-clock and real queue/HTTP tests cover accounting and retained credit/debt. No shared rate or budget is stored in the queue snapshot.

`daemon start --limit R` sets a process-wide body cap using the existing rate grammar; omission is unlimited and restart is required to change it. It is separate from persistent queue `parallel` and each job's own `--limit`. Every read must satisfy both applicable budgets, including probes, redirects, and retries. Changing the concurrency setting never resets shared credit. The full accounting contract is in [daemon-wide rate caps](concurrency.md#daemon-wide-rate-cap).

## Delivery and validation

Follow the [ordered v0.2 plan](roadmap.md#v02-implementation-order), beginning with shared input validation and the strict private queue store. The [daemon validation matrix](testing.md#planned-v02-validation) covers durability, concurrency changes, privacy, crash ambiguity, IPC ownership, and budget composition. The [CLI controls](cli.md#daemon-and-queue) are implemented in the repository build. Document each completed slice and its actual evidence in root STATUS before advancing to release qualification.
