# W1 local web contract

[Documentation index](README.md) · [Product roadmap](roadmap.md#local-web-delivery-order)

**Status:** revised contract, October 22, 2026. W1.1a's earlier version-1 pure model/codec is implemented. The selected [configuration contract](configuration.md) moves editable profiles to one XDG JSON file and uses content tokens; the app ledger must become version 2 before persistence. Config commands, filesystem stores, HTTP, and frontend remain unimplemented.

W1 provides a local downloads/queue app and basic profile defaults around the existing Linux daemon. Preserve the [daemon contract](daemon-and-queue.md), independent standalone commands, and [core integrity criteria](testing.md#core-integrity-acceptance). Native qualification starts on Linux/amd64. Configuration editing alone needs no daemon protocol or queue schema change. Pinning managed transfer-storage choices will require a separately qualified versioned queue/IPC extension before enabling that behavior.

## Current implementation

W1.1a is complete in [internal/app/model.go](../internal/app/model.go), [codec.go](../internal/app/codec.go), and the update guards below. `NewSnapshot` takes explicit instance/profile IDs and an absolute state directory, initializes Default without a destination, and performs no filesystem work. `Snapshot.Validate` checks fields, collection bounds, unique IDs, profile/revision references, saved transfer settings, and encoded size. IDs are 32 lowercase hexadecimal characters; profiles are nonempty, and submissions must be a non-null array. Historical submissions may retain an older directory/settings snapshot after a profile edit.

The pure update guards in [internal/app/update.go](../internal/app/update.go) preserve instance/state-path identity, prevent profile deletion, and keep retained submissions unchanged. New profiles start at revision 1; edited profiles advance exactly once, unchanged profiles retain their revision, and exhausted revisions refuse edits. New submissions reference the current profile revision. Snapshot copies own their slices and nullable directory values; explicit submission removal is permitted without any file action.

`encodeSnapshot` emits newline-terminated version-1 JSON; `decodeSnapshot` requires exact unique fields and declared types/nullability at every level. The codec rejects lossy UTF-8/surrogate input, noncanonical or overflowing decimal strings, trailing JSON, oversized collections, and input or canonical output beyond 4 MiB. Rejections return a fixed error and no partial snapshot/output. Preserve these strict-codec patterns when revising the ledger, rather than plain JSON unmarshalling. This version-1 model is not the selected future disk schema: replace its editable profiles and revision references with the version-2 ledger below before W1.1b/W1.1c.

Model/update/codec tests, bounded fuzzing, and `make check` pass locally. They exercise pure metadata behavior; filesystem storage, HTTP, CLI, and frontend integration remain unimplemented.

## First usable workflow

1. Initialize configuration explicitly, start the daemon with the CLI, then start the separate web process using the same XDG environment. Open its printed loopback address and enter its access token.
2. Show **This computer**, **Downloads**, and **Settings**. Downloads initially shows **All profiles**, including **Unassigned** CLI jobs. Settings contains profiles and daemon status/concurrency. Library, Activity, Automations, and remote-instance selection arrive in later phases.
3. Create or edit a profile, beginning with **Default**. Choose an existing absolute output directory on this computer and transfer defaults. Profile selection is local to the tab; another tab's selection and already submitted jobs do not change.
4. Paste one HTTP(S) link, select a profile, and optionally override its transfer settings or output filename. Preview the resolved destination/settings, then explicitly add the download. Admission returns a job ID; it does not mean the file is complete.
5. Poll retained jobs and use pause, resume, cancel, or forget. Label resume on a failed job **Retry**; it uses the same immutable request and the existing `resume` operation. Closing the tab or web process leaves daemon work running.

The first UI shows output path, profile, state, bytes/known total, worker count, publication outcome, and safe errors. Render unknown values as unknown, including `published: null`; `failed` with `published: true` must identify that verified output exists. Percent complete, file presence, or an empty poll never establishes successful publication. Speeds, ETA, connection inspection, priority/reordering, job editing, aggregate profile caps, and permanent history are outside W1.

Cancel and forget require a clear UI confirmation. Cancel prevents future attempts after the current attempt settles; forget removes an idle queue record and its app association without deleting transfer files. Keep transitional states visible until the daemon reports the result. Backend state checks remain authoritative when a displayed control becomes stale.

## Processes and frontend assets

The planned command is `yoinkdl web --assets <directory> [--port <n>]`. It runs in the foreground, binds only `127.0.0.1`, and defaults to port `0` for an operating-system-assigned port. An explicit port accepts `1..65535`; `0` retains automatic selection. There is no listen-host, remote-daemon, service-install, or automatic-browser-launch option in W1.

The web process loads the initialized configuration, resolves daemon paths once with `daemon.Resolve`, and checks its instance/state binding. It requires an already initialized, private daemon state directory; if missing, fail with explicit daemon-start guidance without creating queue artifacts. Once initialized, the web process may run while the daemon is stopped and show its unavailable state. Only the web process's own metadata directory may be created. Never start, stop, restart, or reset the daemon as a side effect of web startup, browser access, reconnect, or shutdown.

One web process owns `<daemon-state>/web/web.lock` for its lifetime. Config and ledger transactions use separate short-lived `config.lock` and `app.lock` ownership shared with CLI adapters; never keep either solely because the UI is running. The daemon's locks and socket remain independent. SIGINT/SIGTERM stops HTTP admission, settles started metadata writes and bounded IPC calls, closes handlers, then releases web ownership. It does not send daemon `stop`. Do not release metadata ownership while a commit is still running. A killed web process can leave an uncertain submission, but cannot authorize cancellation or replay of a transfer. Exit `0` for help, `2` for invalid options, `1` for startup/runtime failure, and `130` after signal cleanup.

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

The instance ID, selected daemon state path, profile IDs/names, and sparse overrides come only from [config.json](configuration.md#configuration-file). Initialize it explicitly with `config init` before web startup. The instance is labeled **This computer**. Default has no output directory until configured; profiles remain workspaces rather than security tenants.

Use the shared resolver for built-ins, global defaults, selected-profile overrides, and allowed per-add overrides. A profile read view contains `id`, `name`, raw `settings`, effective `output_dir`, `connections`, `retries`, `timeout_ns`, `limit_bps`, storage choices, and per-key `origins`. The response supplies the config-wide `config_token`; it is a 64-character lowercase SHA-256 token, not a numeric profile revision. Names are unique under the selected config contract. Profile create/edit operations require the observed token, and return the new token after durable commit. Unknown/duplicate IDs or names are conflicts. No profile delete/reassignment endpoint is added in W1.

Global edits and valid offline manual edits invalidate old tokens. A lost create/edit reply requires a read before deliberate retry. Historical submissions retain their captured profile ID/name, token, and effective settings when current configuration changes or a profile is manually removed. They remain attributable to that historical profile, not Unassigned. Do not reject the ledger because that ID is absent from today's config.

Check directories on configuration edits/admission/use, not when reading historical metadata. A removed directory leaves profiles readable/editable while new submissions fail. Preserve the existing numeric units and duration/rate grammar. Wire int64 values remain decimal strings; explicit `limit_bps: "0"` means unlimited. Resume is automatic for compatible state; per-add `resume` defaults false and means require an existing checkpoint. W1 web intake never accepts overwrite.

For an omitted filename, reuse `transfer.Normalize`'s URL-basename behavior. An explicit filename is one nonempty basename excluding `.`, `..`, separators, NUL, and reserved artifact names. Join it to the selected profile directory and validate the full absolute output; the browser cannot supply a directory override. Preview does no network or transfer-file mutation and reserves nothing. Engine checks at dispatch/publication remain authoritative.

Preview captures `profile_id`, `config_token`, and complete URL-free normalized settings. Add submits these with the URL and a fresh job ID. Under config ownership, recheck the token, selected profile, output containment, and resolved overrides before saving a binding. Changed global/profile/manual configuration requires a new preview; edited form inputs also invalidate it. Config reads and submitted values never authorize changing an already accepted daemon request.

## Private application metadata

Use a bounded version-2 JSON ledger at `<daemon-state>/web/app.json`, with a persistent transaction lock `app.lock` and separate web-lifetime lock `web.lock`. Require private owned directories/files and the [configuration/ledger lock order](configuration.md#application-metadata-boundary). The app never edits queue snapshots or transfer manifests.

The ledger has exactly `version`, `instance_id`, `state_dir`, and `submissions`. Each submission has exactly `id`, `profile_id`, `profile_name`, `config_token`, and `settings`. Settings are the frozen URL-free request values, including pinned storage policy once qualified. Keep at most 512 records and 4 MiB raw/canonical JSON. IDs are unique and historical names/tokens valid, but no current-profile membership or revision-order rule applies. Initialize an empty ledger only under ownership with matching initialized config and no ambiguous leftovers. Encountered version-1 app files are refused and preserved; no filesystem app store has shipped and no implicit conversion is selected.

Represent int64 values in JSON as canonical decimal strings; bounded counts remain numbers, unknown telemetry remains null, and config tokens remain hex strings. Keep strict field/type/nullability/Unicode/bounds checks, stable instance/state identity, immutable retained submissions, and owned snapshots. Use private exclusive temps, file sync, identity/content revalidation, rename, and directory sync before exposing committed state. Failed or foreign artifacts are never adopted or removed as recovery authority.

Read views combine independent config, ledger, and daemon snapshots; they are not a transaction across owners. Writes acquire config before ledger when both are needed. Keep ledger ownership through a bounded profiled Add/Forget/Dismiss operation, releasing config after the binding is committed and before IPC. This serializes cross-process dismissal against dispatch without blocking later config edits. Busy ledger mutations return an explicit conflict; reads may use validated atomic snapshots without creating locks. Never acquire config ownership while holding ledger ownership.

A pre-rename failure preserves committed state. An uncertain commit or lost artifact identity stops the affected writer/web mutations after cleanup; no rollback write or automatic retry is allowed. Accepted daemon jobs continue independently. Sources, credentials, progress, and duplicate daemon state remain outside this ledger. Browser storage retains only selected profile IDs and unresolved submission IDs; sensitive drafts/tokens stay in memory.

## Submission, association, and recovery

The browser generates a random job ID before sending Add and retains that ID across a failed reply. The API performs the following ordered operation under the shared config/ledger ownership rules:

1. Validate the complete add body, config token, normalized request, metadata capacity, and encoded daemon request size. A known retained submission ID returns a reconciliation conflict without issuing another daemon `add`, even for identical input. Read the current queue and refuse a job ID already present there before creating a binding; never adopt a CLI job through Add. An unavailable preflight read sends no mutation. IDs must be freshly random, not selected from existing jobs; this check is not a transaction against other trusted same-user clients.
2. Durably save the URL-free submission binding before any daemon mutation. If this save fails or is uncertain, do not call the daemon.
3. Call `daemon.Call` once with `add`, that ID, and the normalized immutable request. The source exists in transient request memory and the daemon's existing private source store only. Verify that a successful reply returns the submitted ID.
4. Return the daemon's safe job view, joined to the binding. No second app commit is needed to claim admission: only the successful daemon response proves it. Preserve the binding on an error, timeout, disconnect, or process crash. Never replay saved bindings on startup.

For reads, join current daemon jobs and app submissions by job ID within the selected instance. A match supplies the profile and effective settings. A daemon job with no binding is **Unassigned**, including unprofiled CLI-created jobs; never guess its profile from output path, selected tab, or creation time. Its request/settings remain unavailable through the current safe IPC. It can use the same queue controls. Standalone `download` work is outside the daemon and does not appear here.

A binding absent from a successful queue list appears separately as **Not observed in queue**. It might precede admission, have been rejected, or have been forgotten by another client. Absence does not prove failure or authorize replay. An unavailable daemon instead means the observation is unknown; do not classify every binding as missing. Matching a later job establishes current association/state, not the outcome of every prior control.

After a lost HTTP or IPC mutation acknowledgment, show **Outcome unknown**, the ID where applicable, and a refresh action. Read retries are safe; mutation retries are never automatic, including after reconnect, timeout, page reload, or duplicate form submission. A failed HTTP response body can follow a durable daemon mutation. Do not cancel an admitted job because the response or tab disappeared. IPC cancellation only ends waiting; it is not rollback.

Queue Forget first uses the existing `forget` operation and removes the app binding only after its definitive success. A failed metadata cleanup leaves a recoverable orphan binding, not permission to redo a job. An uncertain forget preserves the binding. A separate explicit **Dismiss submission record** action removes only a currently unmatched local binding, with a warning that it does not cancel a possibly accepted transfer. It requires a successful current queue read and refuses while this process is still dispatching that ID. A delayed job can subsequently appear as Unassigned. As with daemon forget, explicit removal ends retained-ID deduplication; clients still must never replay old requests.

## HTTP interface

All routes below are planned under `/api/v1`. JSON responses have exactly `version: 1`, `result`, and `error`; success has a non-null result and null error, failure has null result and `{code, message, id}`. `id` is the affected ID or null. Codes/messages are fixed and safe; never forward raw parser, filesystem, source, or transport errors. Read and mutation request/response types must be distinct from stored queue requests.

Use the decimal-string integer encoding defined with [application metadata](#private-application-metadata); strict browser-safe encoding is separate from the unchanged daemon wire format.

| Method and path | Input and success result | Existing owner |
|---|---|---|
| `GET /state` | Empty body; instance, config token, profiles, safe daemon/jobs, unmatched submissions, and daemon availability | One `list` call plus owned config/ledger snapshots |
| `POST /profiles` | `config_token`, `id`, `name`, and sparse `settings`; returns the created profile and new config token | Config store |
| `PUT /profiles/{id}` | `config_token`, `name`, and sparse `settings`; returns committed profile and token | Config store with token check |
| `POST /previews` | `url`, `profile_id`, `config_token`, `filename` (null for derived), `overrides` (subset of four defaults), `resume`; returns profile references and effective settings | Pure request resolution/validation |
| `POST /jobs` | `id`, `url`, `profile_id`, `config_token`, `settings` from the preview; returns the safe joined job | Persist binding, then IPC `add` |
| `POST /jobs/{id}/pause` | Empty object; returns the safe joined job | IPC `pause` |
| `POST /jobs/{id}/resume` | Empty object; returns the safe joined job | IPC `resume`, including explicit retry |
| `POST /jobs/{id}/cancel` | Empty object; returns the safe joined job | IPC `cancel` |
| `DELETE /jobs/{id}` | Empty body; returns `id`, `forgotten`, and `association_removed` | IPC `forget`, then app cleanup |
| `DELETE /submissions/{id}` | Empty body; returns `id` and `removed` | Explicit unmatched-binding dismissal |
| `PUT /daemon/parallel` | `parallel` in `1..8`; returns safe daemon status | IPC `set_parallel` |

`GET /state` returns exactly `instance`, `config_token`, `profiles`, `daemon`, `jobs`, `submissions`, and `daemon_error` inside result. Instance contains `id` and `name`. On a good poll, daemon/jobs contain current safe values, submissions contains unmatched bindings, and daemon_error is null. On daemon failure, daemon/jobs are null, submissions contains the saved bindings without a missing-job conclusion, and daemon_error contains a fixed `code`/`message`. A stale queue must never look like a healthy empty queue. App-store failure is an HTTP failure, not a synthesized empty instance.

Safe job fields are the existing `id`, `order`, `state`, `output`, `published`, `error`, `bytes`, `total`, and `workers`, plus `profile_id`, `profile_name`, `config_token`, and `settings` from the binding (all four null for Unassigned). Safe daemon fields remain `state`, `parallel`, `active`, and `limit_bps`. Do not add source URLs/hashes, hostnames, request headers, or inferred transfer statistics. The global daemon cap is read-only in W1; changing it still requires explicit daemon restart through the CLI. There is no web daemon-start/stop endpoint.

Use `201` for a newly admitted job or created profile, and `200` for other successful operations. An accepted pause/cancel response acknowledges intent, not completed draining. Use `400` for invalid input, `401` for missing/invalid credentials, `403` for host/origin refusal, `404` for unknown routes/IDs, `405` for wrong methods, `409` for stale config tokens/state/conflicting or retained submission IDs/capacity, `413` for oversized bodies, `415` for unsupported content type/encoding, `429` for exhausted web request slots, and `503` for unavailable/stopping/persistence/unknown-outcome failures. Preserve the distinction in fixed error codes, especially `outcome_unknown`; HTTP status alone is not retry permission. For Forget where daemon success precedes app cleanup failure, return `200` with `association_removed: false` and retain the unmatched binding for explicit cleanup.

Limit request bodies to 64 KiB, header bytes to 16 KiB, replies to 16 MiB, and active HTTP handlers to 16. At most four IPC calls may be in flight, with at most one web mutation; refuse excess work rather than building an unbounded queue. Retain the daemon's 64 KiB encoded-request bound as a separate check. Profile/submission/queue bounds make one full state response sufficient in W1; no unbounded collection or pagination promise is implied.

Use five-second header/body read deadlines, a fifteen-second response-write deadline, and a thirty-second keep-alive idle deadline. Bound accepted connections as well as handlers to 32. IPC retains its existing deadlines. An HTTP deadline can end waiting/output but cannot roll back a filesystem sync or daemon operation. Strictly reject duplicate/unknown fields, missing required fields, invalid UTF-8, unsupported nulls, trailing JSON, and unsupported versions before mutation. Only the declared nullable fields accept null.

The UI polls at most once per second while visible, with one request in flight per tab. Pause polling when hidden; on visibility/reconnect, read immediately. After read failure, use bounded delays of 1, 2, 4, then 5 seconds; reset after success. Discard older responses following a newer read/control result. Mark the last good view stale and disable queue mutations until a successful refresh. Browser selection/filtering never changes daemon policy. Polls are advisory observations across independent owners, not a transactional snapshot or event history.

## Acceptance and failure cases

These are requirements for the new implementation, not completed test claims. Use local HTTP fixtures, private temporary state/runtime directories, controlled IPC replies, and narrow persistence fault injection.

| ID | Required proof |
|---|---|
| W1-A01 | Clean standalone build/check without Node/assets; absent assets fail only web startup; separate daemon/web lifetime; duplicate web owner refused; web/tab exit leaves exact daemon transfer bytes intact |
| W1-A02 | Missing/wrong/rotated token; foreign/null/missing mutation Origin; alternate Host and rebinding-style host; another localhost port; cross-site forms/preflight/fetch; no private data or side effects; fresh same-origin unlock works |
| W1-A03 | Source/query-token fixture stays out of HTTP replies, app JSON/temp files, browser persistence, static assets, and errors/logs; untrusted profile/path text cannot execute markup; traversal/symlink/static-file exposure refused |
| W1-A04 | Default directory unset; profile create/edit/restart; duplicate names/IDs; stale config tokens in two tabs; all default/override bounds; zero cap and retry count; automatic versus required resume; exact saved effective settings after later edits |
| W1-A05 | Derived and explicit filenames; output stays inside selected directory; no directory creation or preview transfer work; queue size/output conflicts, incompatible sidecars, already-complete output, path replacement, and engine no-overwrite refusal |
| W1-A06 | Binding committed before IPC; failed/uncertain app commit sends nothing; kill after binding commit and after daemon admission; lost/malformed IPC and HTTP replies; duplicate add ID; restart never replays; later matching poll restores profile attribution |
| W1-A07 | CLI jobs stay Unassigned; standalone jobs stay outside the list; empty versus unavailable/stale polls; absent binding/job after CLI forget; explicit dismissal can leave a delayed job Unassigned; no inferred completion or automatic resubmission |
| W1-A08 | All existing state/control rules, late cancel versus publication, failed-but-published and null outcomes, draining slot retention, same-ID explicit retry, global concurrency changes, and terminal source clearing remain authoritative |
| W1-A09 | Config/ledger ownership and modes/link checks, strict schema and integer strings above JavaScript's safe range, stale tokens after global/manual edits, concurrent CLI/web transactions, corrupt/foreign state preservation, temp/write/sync/rename failures, capacity, and fail-stop |
| W1-A10 | Lost profile-edit and forget replies; daemon forget success/app cleanup failure; unmatched-binding dismissal with failed read or dispatch in progress; artifacts always preserved; forgotten IDs are not silently reused by the UI |
| W1-A11 | Request/header/reply/client limits, slow bodies/readers, multiple polling tabs, concurrent CLI access, write deadline during accepted mutation, shutdown with commit/IPC in flight; no unbounded goroutine/response backlog or daemon stall |
| W1-A12 | Actual Vue flow in Firefox: unlock, configure two profiles, preview/add, switch/filter, pause/resume/retry/cancel/forget, refresh after lost response and daemon restart; keyboard access, mobile width, frontend typecheck/tests/build, and focused engine/IPC plus full Go gate |

Existing evidence to preserve includes `TestDaemonClientNeverStartsOrCreatesState`, `TestDaemonClientLostReplyIsUncertainOnlyForMutations`, `TestProtocolReplyShapesAndPrivacy`, `TestQueueDisconnectedAddSurvivesWithoutAliasingCaller`, `TestRecoveryPreservesUnknownOutcomes`, and the request normalization tests. They establish reusable backend behavior; they do not qualify a web adapter or browser authentication.

## Delivery order

Implement the [selected configuration foundation](configuration.md#delivery-and-validation) first, revise the pure app ledger to version 2, then retain the W1.1b/W1.1c ownership/persistence split. Finish the transfer-storage contract before its integration. Configuration, filesystem persistence, request/IPC adapters, and frontend remain separate reviewable slices.

| Step | Focus of one reviewable change, with its tests and docs |
|---|---|
| W1.1a — earlier baseline complete | Pure version-1 app metadata model and strict codec: IDs, initial profile defaults, URL-free bindings, references, decimal strings, bounds, and immutable snapshot validation. Revise to the selected version-2 ledger before disk work. |
| W1.1b | Linux app-directory, separate web/transaction locks, and strict version-2 existing-ledger loading, including foreign/corrupt artifact preservation. |
| W1.1c | Atomic initialization/saves, owned snapshots, canonical identity checks, commit failures, and fail-stop recovery. |
| W1.2a | HTTP bearer/host/origin boundary and request/connection limits, tested without frontend or daemon mutations. |
| W1.2b | Read-only `/state` adapter over the app store and safe IPC, including Unassigned jobs, unavailable reads, and exact integer projection. |
| W1.2c | Explicit foreground `web` CLI, validated static assets, independent shutdown, and failed startup/output cleanup. |
| W1.2d | Locked Vue/TypeScript/Vite scaffold, explicit build/check targets, and unlock shell; keep the Go-only build independent. |
| W1.2e | Read-only Downloads UI, polling/stale behavior, profile filters, and daemon status. |
| W1.3a | Profile create/edit API over the shared config store with token checks, plus pure effective-setting/filename previews. |
| W1.3b | Profile Settings and preview form UI; Add remains disabled. |
| W1.3c | Durable binding before one daemon Add, collision checks, unknown replies, crash/restart behavior, and no replay. |
| W1.3d | Enable Add in the UI with submitted-ID retention, stale-config handling, and explicit unknown-outcome recovery. |
| W1.4a | Queue pause/resume/cancel and parallel adapters with state/publication-race coverage; wire corresponding UI controls in a following focused commit. |
| W1.4b | Forget/dismiss metadata cleanup and partial-failure semantics; add the UI confirmations/recovery in a following focused commit. |
| W1.5 | Remaining process/browser acceptance and usability fixes, each committed by the behavior it qualifies; run the full frontend and Go gates and update current user docs. |

The owner requested smaller commits. These steps set dependency order, not permission to bundle all work in a row: split further when responsibilities differ. Keep each commit buildable, include its behavior tests/docs and STATUS update, and leave unfinished controls disabled. Never combine the metadata model, filesystem persistence, HTTP boundary, and frontend into one commit. No push or release step is part of this sequence. W2–W4 and later Firefox/hosted tracks retain their [roadmap order](roadmap.md#local-web-delivery-order).
