# W1 local web contract

[Documentation index](README.md) · [Product roadmap](roadmap.md#local-web-delivery-order)

**Status:** selected implementation contract, October 20, 2026. The web command, HTTP API, application store, and frontend described here are not implemented. The remaining W1 contracts are being defined before implementation.

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

## Acceptance and failure cases

These are requirements for the new implementation, not completed test claims. Use local HTTP fixtures, private temporary state/runtime directories, controlled IPC replies, and narrow persistence fault injection.

| ID | Required proof |
|---|---|
| W1-A01 | Clean standalone build/check without Node/assets; absent assets fail only web startup; separate daemon/web lifetime; duplicate web owner refused; web/tab exit leaves exact daemon transfer bytes intact |
| W1-A02 | Missing/wrong/rotated token; foreign/null/missing mutation Origin; alternate Host and rebinding-style host; another localhost port; cross-site forms/preflight/fetch; no private data or side effects; fresh same-origin unlock works |
| W1-A03 | Source/query-token fixture stays out of HTTP replies, app JSON/temp files, browser persistence, static assets, and errors/logs; untrusted profile/path text cannot execute markup; traversal/symlink/static-file exposure refused |
