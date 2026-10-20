# W1 local web contract

[Documentation index](README.md) · [Product roadmap](roadmap.md#local-web-delivery-order)

**Status:** selected implementation contract, October 20, 2026. The web command, HTTP API, application store, and frontend described here are not implemented. The W1 behavior contract is defined; the implementation order is being split into focused changes.

W1 provides a local downloads/queue app and basic profile defaults around the existing Linux daemon. Preserve the [daemon contract](daemon-and-queue.md), independent standalone commands, and [core integrity criteria](testing.md#core-integrity-acceptance). Native qualification starts on Linux/amd64. No daemon protocol or queue schema change is required.

## First usable workflow

1. Start the daemon explicitly with the existing CLI, then start the separate web process using the same XDG environment. Open its printed loopback address and enter its access token.
2. Show **This computer**, **Downloads**, and **Settings**. Downloads initially shows **All profiles**, including **Unassigned** CLI jobs. Settings contains profiles and daemon status/concurrency. Library, Activity, Automations, and remote-instance selection arrive in later phases.
3. Create or edit a profile, beginning with **Default**. Choose an existing absolute output directory on this computer and transfer defaults. Profile selection is local to the tab; another tab's selection and already submitted jobs do not change.
4. Paste one HTTP(S) link, select a profile, and optionally override its transfer settings or output filename. Preview the resolved destination/settings, then explicitly add the download. Admission returns a job ID; it does not mean the file is complete.
5. Poll retained jobs and use pause, resume, cancel, or forget. Label resume on a failed job **Retry**; it uses the same immutable request and the existing `resume` operation. Closing the tab or web process leaves daemon work running.

The first UI shows output path, profile, state, bytes/known total, worker count, publication outcome, and safe errors. Render unknown values as unknown, including `published: null`; `failed` with `published: true` must identify that verified output exists. Percent complete, file presence, or an empty poll never establishes successful publication. Speeds, ETA, connection inspection, priority/reordering, job editing, aggregate profile caps, and permanent history are outside W1.

Cancel and forget require a clear UI confirmation. Cancel prevents future attempts after the current attempt settles; forget removes an idle queue record and its app association without deleting transfer files. Keep transitional states visible until the daemon reports the result. Backend state checks remain authoritative when a displayed control becomes stale.

## Processes and frontend assets

The planned command is `yoinkdl web --assets <directory> [--port <n>]`. It runs in the foreground, binds only `127.0.0.1`, and defaults to port `0` for an operating-system-assigned port. An explicit port accepts `1..65535`; `0` retains automatic selection. There is no listen-host, remote-daemon, service-install, or automatic-browser-launch option in W1.

The web process resolves daemon paths once with `daemon.Resolve`. It requires an already initialized, private daemon state directory; if missing, fail with explicit daemon-start guidance without creating queue artifacts. Once initialized, the web process may run while the daemon is stopped and show its unavailable state. Only the web process's own metadata directory may be created. Never start, stop, restart, or reset the daemon as a side effect of web startup, browser access, reconnect, or shutdown.

One web process owns the app store per selected state directory, using its own lock. The daemon's locks and socket remain independent. SIGINT/SIGTERM stops HTTP admission, settles started metadata writes and bounded IPC calls, closes handlers, then releases the app lock. It does not send daemon `stop`. Do not release metadata ownership while a commit is still running. A killed web process can leave an uncertain submission, but cannot authorize cancellation or replay of a transfer. Exit `0` for help, `2` for invalid options, `1` for startup/runtime failure, and `130` after signal cleanup.

Use Vue 3 + TypeScript with Vite under planned `web/`, pnpm with a committed lockfile, and explicit frontend typecheck/test/build scripts. Pin supported tool/dependency versions when scaffolding. Vue Flow and the Playwright browser worker are later features, not W1 runtime dependencies.

Vite produces static `web/dist` assets; the Go process serves those assets and `/api/v1` from the same origin. Node/pnpm is needed to build the frontend, not to serve it. `--assets` is required and resolved against the launching shell once. Validate the build entry/assets before listening; missing assets fail clearly, with no placeholder page or CDN fallback. Serve only the validated build files, with no directory listing, symlink escape, source maps, repository files, metadata, or downloads exposed by the static handler.

Keep `make build`, `make check`, and standalone packaging independent of Node and generated assets. Add explicit frontend build/check targets during scaffolding; dependency installation is a separate locked step. The initial development loop builds or watches assets and opens the Go origin. A Vite development server/proxy and relaxed development CORS are not part of W1. Distribution/embedded-asset packaging can be decided later without blocking this runnable local layout. [Vite documents static production builds](https://vite.dev/guide/build); the same-origin serving and optional-build boundaries are Yoink choices.

## Local authentication and origin checks

At each web launch generate 32 random bytes, encode them as unpadded base64url, and print the address and token separately to the invoking terminal. Fail startup if the initial output cannot be written. The token is an explicitly disclosed local credential, never part of a URL, command argument, persistent app file, access log, or error. Restart rotates it. The unlock form keeps it only in tab memory and sends `Authorization: Bearer <token>`; reload requires unlocking again. W1 uses no auth cookies, persistent browser token storage, or account model.

The static unlock shell contains no private state. Every `/api/v1` route, including reads and previews, requires the bearer token with constant-time comparison. Authentication alone does not replace these checks:

- Require the exact bound `Host` value `127.0.0.1:<port>` on all requests. Reject alternate hosts, malformed/multiple host values, and absolute-form proxy requests. Never trust forwarded headers or offer proxying/redirects to a caller-selected destination.
- Reject a present `Origin` unless it exactly equals the printed origin, including scheme and port; reject `null`. Every non-GET API request requires that exact origin. Authenticated GETs may omit it because same-origin reads need not send it.
- If `Sec-Fetch-Site` is present on an API request, require `same-origin`. Missing fetch metadata does not bypass bearer/origin checks. Do not enable CORS or authorize preflight requests. Simple forms, cross-origin reads, and requests from another loopback port must have no private response or mutation effects.
- Require `application/json` on routes with request bodies; declared bodyless GET/DELETE routes accept no body. Reject form encodings, unknown methods/routes/query parameters, and unsupported content encodings before dispatch. The API has no redirect, JSONP, WebSocket, or event-stream path.

Serve `Cache-Control: no-store` for HTML/API responses, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, and `Cross-Origin-Resource-Policy: same-origin`. Use a content security policy allowing only same-origin scripts, styles, images, fonts, and connections, with no inline/eval scripts, framing, base-URI changes, or form navigation. Render paths, profile names, and errors as text, never HTML. Do not install a service worker or third-party analytics/resources.

These checks protect the local browser boundary; profiles are not security tenants and same-user processes remain trusted. Private/loopback source URLs keep the existing owner-directed downloader policy. Network binding, forwarding through a reverse proxy, extension access, and remote authentication require the later L1/L2 contracts. Browser behavior references: [Authorization](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Authorization), [CORS/preflight](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS), and [Sec-Fetch-Site](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Sec-Fetch-Site).

## Profiles and effective settings

An app instance gets a random stable 32-character lowercase hex ID and the display label **This computer**. Bind its metadata to the resolved daemon state directory; refuse a stored path mismatch rather than silently attaching it to another queue. A profile has its own random ID, a revision, a name, an output directory, and defaults. It is a workspace within this instance, not a user account.

Initialize one **Default** profile with `output_dir: null`. Require the owner to choose a directory before adding from it; never infer a download directory from the web process's cwd or create one silently. Allow up to 32 profiles. Names are trimmed nonempty UTF-8 text of at most 80 bytes without control characters. IDs establish identity; duplicate display names are permitted and distinguished in the UI. W1 supports create/edit/select, with profile deletion and reassignment of existing jobs deferred.

| Profile field | Contract |
|---|---|
| `id`, `revision`, `name` | Stable ID; positive monotonic revision; display name |
| `output_dir` | Null until configured, otherwise a clean absolute existing directory; output paths remain subject to engine checks |
| `connections` | Integer `1..32`, default `4` |
| `retries` | Integer `0..10`, default `3` |
| `timeout_ns` | Positive duration in nanoseconds, default `30000000000`; network inactivity, not a total deadline |
| `limit_bps` | Nonnegative per-download body cap, default `0` for unlimited; not a shared profile budget |

Revision starts at `1`, increases only after a durable edit, and never wraps. Profile edits require the observed revision and reject stale writes. Profile creation takes a client-generated ID; a duplicate ID is a conflict, not another profile. After a lost edit/create acknowledgment, read back the profile before any deliberate retry.

Check directory existence when configuring or submitting, not when loading historical metadata. A removed directory must leave profiles readable/editable while new submissions fail safely. Retained submissions keep their original output/settings even when the profile now points elsewhere.

Resolve built-in defaults into complete profile defaults, then apply explicit per-add overrides. Input forms accept the existing duration/rate grammar; wire/storage settings use the normalized numeric units above. An override's omission means inherit, while explicit `limit_bps: "0"` means unlimited. Resume automatically reuses compatible checkpoints as today; the per-add `resume` boolean defaults to false and means **require an existing checkpoint** when true. It is not a switch for disabling safe reuse. W1 never accepts overwrite.

For an omitted filename, use `transfer.Normalize`'s existing URL-basename behavior. An explicit filename is one nonempty basename, excluding `.`, `..`, `/`, `\`, NUL, and reserved transfer-artifact names. Join it to the selected profile directory and validate the full absolute output with `queue.NewJob`; do not accept a caller-supplied directory override. Preview does no network or transfer-file work and does not reserve the path. Engine checks at actual dispatch/publication still decide existence, ownership, compatible resume, and no-overwrite outcomes.

Preview captures `profile_id`, `profile_revision`, and the complete URL-free effective settings: `output`, `connections`, `retries`, `timeout_ns`, `limit_bps`, and `resume`. Add submits those settings with the URL. The API validates the revision and all settings again, including that the output is directly inside that profile's configured directory. A stale revision forces a new preview; edited form inputs invalidate the displayed preview. Once admitted, profile switching/editing never changes the daemon's stored request or the saved effective snapshot.

## Private application metadata

Use a bounded version-1 JSON snapshot at `<daemon-state>/web/app.json` with a persistent `<daemon-state>/web/app.lock`. The directory is `0700`, files are `0600`, and ownership must match the effective user. This small W1 store needs no database or TOML loader. Do not open or edit `queue.json`, transfer manifests, or the daemon's locks from the app.

The snapshot has exactly `version`, `instance_id`, `state_dir`, `profiles`, and `submissions`. Profiles use the fields above. A submission has exactly `id`, `profile_id`, `profile_revision`, and `settings`, where settings are the URL-free effective snapshot. All referenced profiles must exist; a saved profile revision may be older than its current revision. IDs must be unique within each collection. Permit at most 512 submission records and a 4 MiB encoded snapshot. This covers the daemon's 256 retained jobs plus bounded unresolved submissions; exhaustion refuses new adds without evicting metadata or queue jobs.

Represent all 64-bit integers as canonical decimal strings in the app snapshot and web API: revisions/orders are positive, byte counters/rates are nonnegative, and values must fit signed 64 bits. Null remains null for unknown telemetry. Bounded values (`version`, `connections`, `retries`, `parallel`, `active`, `workers`) remain JSON numbers. Convert validated IPC integers in Go before JSON encoding so the browser never rounds file sizes or FIFO orders. This does not change existing CLI/IPC encoding.

Never persist sources, query tokens, headers, cookies, auth tokens, request hashes, progress, or copies of daemon job state in this file. A submission is a durable association/intent, not a queue, history entry, or proof of admission. Browser storage may retain only selected profile IDs and unresolved submission IDs for reconnect; URLs, settings drafts, tokens, and private views stay in memory. Clear the URL draft after an accepted or uncertain add, and never reflect it in a response.

Strictly validate versions, field sets, duplicate keys, nullability, UTF-8, numeric bounds, path bindings, collection sizes, and references on load and before save. Reject unsafe owners/modes, symlinks, hard-linked mutable files, corrupt snapshots, or changed canonical artifacts without repairing/removing them. One serialized writer owns candidate snapshots; reads return owned copies. Preserve instance/profile identity, monotonic profile revisions, and immutable retained submission bindings across saves. Write an exclusively created private temp file, sync it, rename, sync the directory, then acknowledge/expose the committed state. Check canonical file and lock identity before replacement. Keep failed/foreign artifacts intact; never promote leftover temps or initialize empty state over ambiguous leftovers.

A pre-rename failure preserves the old committed snapshot. A post-rename directory-sync failure or detected ownership change stops web mutations and closes the web process after handler cleanup; it never writes an older snapshot back. Restart validates canonical state under the lock. Previously accepted daemon jobs continue independently. Reuse established storage patterns where their boundaries fit, without coupling app metadata lifetime to the queue owner. W2 must define any schema migration before adding library/history or replacing this store.

## Submission, association, and recovery

The browser generates a random job ID before sending Add and retains that ID across a failed reply. The API performs the following ordered operation under its app mutation ownership:

1. Validate the complete add body, profile revision, normalized request, metadata capacity, and encoded daemon request size. A known retained submission ID returns a reconciliation conflict without issuing another daemon `add`, even for identical input. Read the current queue and refuse a job ID already present there before creating a binding; never adopt a CLI job through Add. An unavailable preflight read sends no mutation. IDs must be freshly random, not selected from existing jobs; this check is not a transaction against other trusted same-user clients.
2. Durably save the URL-free submission binding before any daemon mutation. If this save fails or is uncertain, do not call the daemon.
3. Call `daemon.Call` once with `add`, that ID, and the normalized immutable request. The source exists in transient request memory and the daemon's existing private source store only. Verify that a successful reply returns the submitted ID.
4. Return the daemon's safe job view, joined to the binding. No second app commit is needed to claim admission: only the successful daemon response proves it. Preserve the binding on an error, timeout, disconnect, or process crash. Never replay saved bindings on startup.

For reads, join current daemon jobs and app submissions by job ID within the selected instance. A match supplies the profile and effective settings. A daemon job with no binding is **Unassigned**, including CLI-created jobs; never guess its profile from output path, selected tab, or creation time. Its request/settings remain unavailable through the current safe IPC. It can use the same queue controls. Standalone `download` work is outside the daemon and does not appear here.

A binding absent from a successful queue list appears separately as **Not observed in queue**. It might precede admission, have been rejected, or have been forgotten by another client. Absence does not prove failure or authorize replay. An unavailable daemon instead means the observation is unknown; do not classify every binding as missing. Matching a later job establishes current association/state, not the outcome of every prior control.

After a lost HTTP or IPC mutation acknowledgment, show **Outcome unknown**, the ID where applicable, and a refresh action. Read retries are safe; mutation retries are never automatic, including after reconnect, timeout, page reload, or duplicate form submission. A failed HTTP response body can follow a durable daemon mutation. Do not cancel an admitted job because the response or tab disappeared. IPC cancellation only ends waiting; it is not rollback.

Queue Forget first uses the existing `forget` operation and removes the app binding only after its definitive success. A failed metadata cleanup leaves a recoverable orphan binding, not permission to redo a job. An uncertain forget preserves the binding. A separate explicit **Dismiss submission record** action removes only a currently unmatched local binding, with a warning that it does not cancel a possibly accepted transfer. It requires a successful current queue read and refuses while this process is still dispatching that ID. A delayed job can subsequently appear as Unassigned. As with daemon forget, explicit removal ends retained-ID deduplication; clients still must never replay old requests.

## HTTP interface

All routes below are planned under `/api/v1`. JSON responses have exactly `version: 1`, `result`, and `error`; success has a non-null result and null error, failure has null result and `{code, message, id}`. `id` is the affected ID or null. Codes/messages are fixed and safe; never forward raw parser, filesystem, source, or transport errors. Read and mutation request/response types must be distinct from stored queue requests.

Use the decimal-string integer encoding defined with [application metadata](#private-application-metadata); strict browser-safe encoding is separate from the unchanged daemon wire format.

| Method and path | Input and success result | Existing owner |
|---|---|---|
| `GET /state` | Empty body; instance, profiles, safe daemon/jobs, unmatched submissions, and daemon availability | One `list` call plus an owned app snapshot |
| `POST /profiles` | `id`, `name`, `output_dir`, and all four profile transfer defaults; returns the created profile at revision `"1"` | App store |
| `PUT /profiles/{id}` | `revision`, `name`, `output_dir`, and all four defaults; returns the committed profile | App store with revision check |
| `POST /previews` | `url`, `profile_id`, `profile_revision`, `filename` (null for derived), `overrides` (subset of four defaults), `resume`; returns profile references and effective settings | Pure request resolution/validation |
| `POST /jobs` | `id`, `url`, `profile_id`, `profile_revision`, `settings` from the preview; returns the safe joined job | Persist binding, then IPC `add` |
| `POST /jobs/{id}/pause` | Empty object; returns the safe joined job | IPC `pause` |
| `POST /jobs/{id}/resume` | Empty object; returns the safe joined job | IPC `resume`, including explicit retry |
| `POST /jobs/{id}/cancel` | Empty object; returns the safe joined job | IPC `cancel` |
| `DELETE /jobs/{id}` | Empty body; returns `id`, `forgotten`, and `association_removed` | IPC `forget`, then app cleanup |
| `DELETE /submissions/{id}` | Empty body; returns `id` and `removed` | Explicit unmatched-binding dismissal |
| `PUT /daemon/parallel` | `parallel` in `1..8`; returns safe daemon status | IPC `set_parallel` |

`GET /state` returns exactly `instance`, `profiles`, `daemon`, `jobs`, `submissions`, and `daemon_error` inside result. Instance contains `id` and `name`. On a good poll, daemon/jobs contain current safe values, submissions contains unmatched bindings, and daemon_error is null. On daemon failure, daemon/jobs are null, submissions contains the saved bindings without a missing-job conclusion, and daemon_error contains a fixed `code`/`message`. A stale queue must never look like a healthy empty queue. App-store failure is an HTTP failure, not a synthesized empty instance.

Safe job fields are the existing `id`, `order`, `state`, `output`, `published`, `error`, `bytes`, `total`, and `workers`, plus `profile_id`, `profile_revision`, and `settings` from the binding (all three null for Unassigned). Safe daemon fields remain `state`, `parallel`, `active`, and `limit_bps`. Do not add source URLs/hashes, hostnames, request headers, or inferred transfer statistics. The global daemon cap is read-only in W1; changing it still requires explicit daemon restart through the CLI. There is no web daemon-start/stop endpoint.

Use `201` for a newly admitted job or created profile, and `200` for other successful operations. An accepted pause/cancel response acknowledges intent, not completed draining. Use `400` for invalid input, `401` for missing/invalid credentials, `403` for host/origin refusal, `404` for unknown routes/IDs, `405` for wrong methods, `409` for stale revisions/state/conflicting or retained submission IDs/capacity, `413` for oversized bodies, `415` for unsupported content type/encoding, `429` for exhausted web request slots, and `503` for unavailable/stopping/persistence/unknown-outcome failures. Preserve the distinction in fixed error codes, especially `outcome_unknown`; HTTP status alone is not retry permission. For Forget where daemon success precedes app cleanup failure, return `200` with `association_removed: false` and retain the unmatched binding for explicit cleanup.

Limit request bodies to 64 KiB, header bytes to 16 KiB, replies to 16 MiB, and active HTTP handlers to 16. At most four IPC calls may be in flight, with at most one web mutation; refuse excess work rather than building an unbounded queue. Retain the daemon's 64 KiB encoded-request bound as a separate check. Profile/submission/queue bounds make one full state response sufficient in W1; no unbounded collection or pagination promise is implied.

Use five-second header/body read deadlines, a fifteen-second response-write deadline, and a thirty-second keep-alive idle deadline. Bound accepted connections as well as handlers to 32. IPC retains its existing deadlines. An HTTP deadline can end waiting/output but cannot roll back a filesystem sync or daemon operation. Strictly reject duplicate/unknown fields, missing required fields, invalid UTF-8, unsupported nulls, trailing JSON, and unsupported versions before mutation. Only the declared nullable fields accept null.

The UI polls at most once per second while visible, with one request in flight per tab. Pause polling when hidden; on visibility/reconnect, read immediately. After read failure, use bounded delays of 1, 2, 4, then 5 seconds; reset after success. Discard older responses following a newer read/control result. Mark the last good view stale and disable queue mutations until a successful refresh. Browser selection/filtering never changes daemon policy. Polls are advisory observations across two owners, not a transactional snapshot or event history.

## Acceptance and failure cases

These are requirements for the new implementation, not completed test claims. Use local HTTP fixtures, private temporary state/runtime directories, controlled IPC replies, and narrow persistence fault injection.

| ID | Required proof |
|---|---|
| W1-A01 | Clean standalone build/check without Node/assets; absent assets fail only web startup; separate daemon/web lifetime; duplicate web owner refused; web/tab exit leaves exact daemon transfer bytes intact |
| W1-A02 | Missing/wrong/rotated token; foreign/null/missing mutation Origin; alternate Host and rebinding-style host; another localhost port; cross-site forms/preflight/fetch; no private data or side effects; fresh same-origin unlock works |
| W1-A03 | Source/query-token fixture stays out of HTTP replies, app JSON/temp files, browser persistence, static assets, and errors/logs; untrusted profile/path text cannot execute markup; traversal/symlink/static-file exposure refused |
| W1-A04 | Default directory unset; profile create/edit/restart; duplicate names/IDs; stale revisions in two tabs; all default/override bounds; zero cap and retry count; automatic versus required resume; exact saved effective settings after later edits |
| W1-A05 | Derived and explicit filenames; output stays inside selected directory; no directory creation or preview transfer work; queue size/output conflicts, incompatible sidecars, already-complete output, path replacement, and engine no-overwrite refusal |
| W1-A06 | Binding committed before IPC; failed/uncertain app commit sends nothing; kill after binding commit and after daemon admission; lost/malformed IPC and HTTP replies; duplicate add ID; restart never replays; later matching poll restores profile attribution |
| W1-A07 | CLI jobs stay Unassigned; standalone jobs stay outside the list; empty versus unavailable/stale polls; absent binding/job after CLI forget; explicit dismissal can leave a delayed job Unassigned; no inferred completion or automatic resubmission |
| W1-A08 | All existing state/control rules, late cancel versus publication, failed-but-published and null outcomes, draining slot retention, same-ID explicit retry, global concurrency changes, and terminal source clearing remain authoritative |
| W1-A09 | App ownership/modes/link checks, strict schema and integer-string boundaries above JavaScript's safe range, corrupt/foreign state preservation, temp/write/sync/rename failures, capacity, revision exhaustion, and store fail-stop |
| W1-A10 | Lost profile-edit and forget replies; daemon forget success/app cleanup failure; unmatched-binding dismissal with failed read or dispatch in progress; artifacts always preserved; forgotten IDs are not silently reused by the UI |
| W1-A11 | Request/header/reply/client limits, slow bodies/readers, multiple polling tabs, concurrent CLI access, write deadline during accepted mutation, shutdown with commit/IPC in flight; no unbounded goroutine/response backlog or daemon stall |
| W1-A12 | Actual Vue flow in Firefox: unlock, configure two profiles, preview/add, switch/filter, pause/resume/retry/cancel/forget, refresh after lost response and daemon restart; keyboard access, mobile width, frontend typecheck/tests/build, and focused engine/IPC plus full Go gate |

Existing evidence to preserve includes `TestDaemonClientNeverStartsOrCreatesState`, `TestDaemonClientLostReplyIsUncertainOnlyForMutations`, `TestProtocolReplyShapesAndPrivacy`, `TestQueueDisconnectedAddSurvivesWithoutAliasingCaller`, `TestRecoveryPreservesUnknownOutcomes`, and the request normalization tests. They establish reusable backend behavior; they do not qualify a web adapter or browser authentication.
