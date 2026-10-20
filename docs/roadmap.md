# Product roadmap

[Documentation index](README.md)

Build a local-first web application around Yoink's existing download engine, then add automation and later browser/server integration. This roadmap tracks feature dependencies and useful increments; it does not assign version targets or publication steps. The TUI remains canceled.

## Implemented foundation

The [standalone CLI](cli.md) already provides HTTP(S) transfers, safe sequential/parallel resume, bounded retries, progress, rate caps, cancellation, and verified file publication. The [Linux daemon and queue](daemon-and-queue.md) add private local IPC, durable admission/control, configurable concurrency, shared budgets, and conservative recovery.

Preserve those responsibilities as the app grows. The [core integrity criteria](testing.md#core-integrity-acceptance) and existing tests remain the baseline; new interfaces must not bypass response validation, ownership, checkpoints, or overwrite protection.

## Local web application

**Status:** the owner approved a local-first web application direction on October 20, 2026. The [W1 local contract](local-web.md) now defines downloads/queue management and basic profiles around the existing Go daemon; implementation has not started. Hosted operation and browser integration come later. The TUI remains canceled.

W1–W4 are implementation phases. The capability list describes the long-term direction; define a concrete slice and its acceptance tests before adding each behavior.

### Instance and profile model

An **instance** identifies where Yoink runs and where files are stored: initially **This computer**, later an explicitly configured **Home server**. A **profile** is a workspace inside that instance, such as **Pato**, with its own defaults and associations. Instance selection and profile switching are separate operations; a profile name does not imply a separate user account or security tenant.

Switching profiles must not restart the daemon, migrate files between machines, or silently change accepted jobs. Capture effective settings when submitting a job. Distinguish a workflow definition/version, an execution run, its download jobs, and the resulting library files so their histories can be correlated.

### Planned application areas

| Area | Planned capabilities |
|---|---|
| Downloads & Queue | Add/configure jobs; show live progress, speeds, states, connection activity, and statistics; pause/resume/cancel/retry; organize jobs and eventually change priorities/order |
| Profiles | Independent defaults, output directories, connection preferences, bandwidth policies, rules, schedules, workflows, and browser-session associations; switch without restarting the daemon |
| Download Rules | Match URLs/domains, extensions, MIME types, sizes, and other metadata; choose destinations, names, profiles, priorities, and settings with deterministic precedence and a preview of resolved choices |
| Download Library | Browse/search/filter completed files; tags, collections, metadata, source tracking, duplicate detection, and explicit file actions appropriate to the selected instance |
| Schedulers | One-time/recurring execution and event triggers for downloads or workflows; execution history, retry policies, timezones, missed-run handling, and overlap policies |
| Visual Grabber | Drag, drop, connect, and configure browser-action, scraping, link-extraction, and download nodes; saved/reusable workflows, variables, conditional branches, loops, subflows, and live execution debugging |
| Activity & History | Searchable download, workflow, schedule, and daemon activity with linked run/job/file identities, detailed logs, diagnostics, errors, and transfer statistics |
| Settings & Daemon Control | Application/profile settings, daemon health, active work, concurrency/bandwidth settings, aggregate bandwidth usage, and qualified connection/resource telemetry |
| Import & Export | Versioned download lists, profiles, workflows/templates, rules, and settings; credential references and destination-path remapping for portability |
| Browser Integration — later | Firefox extension first: explicit link handoff and opt-in interception of supported downloads to a selected instance/profile |

Organize the UI around **Downloads**, **Library**, **Automations** (Grabber, schedules, rules), **Activity**, and **Settings**, with instance/profile selectors above them. Import/export and browser integration can live in their relevant settings/actions rather than requiring separate primary destinations.

### Technology and ownership

The selected frontend direction is **Vue 3 + TypeScript**, with **Vue Flow selected for the Visual Grabber editor**. Vue Flow supplies node/edge interaction and visualization; workflow validation, execution, retries, persistence, scheduling, and debugging state remain separate application responsibilities. Do not make executable workflow definitions depend on the editor's transient component state or layout.

Plan an optional **Go application API** that serves the frontend and talks to the existing private daemon IPC. The daemon remains authoritative for admission, byte transfer, checkpoints, locking, and final publication. W1 selects a separate foreground web process, static Vue/Vite assets, and a private bounded JSON store for profiles and URL-free submission bindings. Rules, workflow versions, runs, library, and history schemas remain later contracts. Preserve independent standalone CLI use, and do not edit private queue snapshots from the web layer.

Use an optional **Playwright browser worker** for the browser-automation direction. Browser processes and their resource limits are separate from download workers and bandwidth accounting. Go can serve built frontend assets without a Node web server; browser automation introduces its own runtime/browser installation only when that feature is enabled. The [architecture boundaries](architecture.md#planned-web-application) describe these planned components; none are implemented.

### Local web delivery order

| Phase | Outcome | Required boundary before advancing |
|---|---|---|
| W1 | Local web app for downloads/queue and basic profile defaults | Implement the [selected contract and ordered slices](local-web.md#delivery-order): private profile/intent metadata, authenticated local process/read-only UI, preview/add, controls/recovery, then full acceptance; retain standalone compatibility |
| W2 | Durable library/activity, download rules, and richer job/profile management | Define metadata ownership/retention, rule stages/precedence, duplicate identity, and portable definitions; qualify any daemon changes for priorities, reordering, job edits, aggregate profile limits, and new telemetry |
| W3 | Schedules and a small typed workflow/browser runner | Durable run identity, missed/overlapping execution policies, safe retries/deduplication, and three representative browser-to-downloader handoff cases; execution must continue independently of the UI tab |
| W4 | Vue Flow Visual Grabber and richer workflow authoring/debugging | Versioned definitions independent of canvas state, validated branches/variables/loops/subflows, bounded execution, cancellation ownership, diagnostics, and manual/scheduled parity |

Capture representative browser-download journeys early enough to test authenticated requests and browser-generated files before investing in the full editor. Ordinary HTTP(S) links, expiring/authenticated requests, and browser-managed files are distinct capabilities. Unsupported handoffs must be explicit; a separate browser download path does not inherit the Go engine's resume/publication guarantees automatically.

The [current daemon contract](daemon-and-queue.md) already supports safe polling and core queue controls. Priorities/reordering, mutable job settings, aggregate profile budgets, first-class speed/connection/resource metrics, and permanent history are **not implemented**. Worker counts are not a connection inspector, and the 256-record retained queue is not a library/history database. Each extension needs its own compatible contract and tests.

### Later browser integration and hosted operation

The owner explicitly deferred the extension and server-target selection. They are not W1 requirements or prerequisites for building the local app:

| Later track | Intended behavior | Dependencies |
|---|---|---|
| L1 — Firefox extension | **Send this link to Yoink** to an explicitly selected local instance/profile, followed by opt-in automatic interception of supported downloads; explore a native bridge or authenticated local communication | Stable intake/profile/rule contracts from the local app; Firefox permissions, safe takeover/recovery, duplicate handling, and credential-transfer policy |
| L2 — Self-hosted instance | Use the same application model on an explicitly configured server; let the web UI/extension choose a remote instance and profile | Qualified local behavior, remote authentication/transport, instance discovery/pairing, and server-side file/credential ownership |

**Firefox is the first browser target**, matching the owner's daily browser. Chromium support can be considered later. Include an interception toggle and an explicit destination instance/profile, while retaining manual send-to actions.

Firefox's [downloads API](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/downloads) provides download events and pause/cancel/resume controls; [webRequest](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/webRequest) supports request interception with the appropriate API/host permissions. Choose the takeover mechanism during L1 design. The [onCreated event](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/downloads/onCreated) observes a download after it begins, so capture does not guarantee zero browser-side bytes. Qualify cancellation/admission ordering, downloads completing during handoff, unavailable bridges/daemons, lost acknowledgments, and duplicate events. Unsupported downloads must remain in Firefox; failed or uncertain handoffs need explicit recovery without silent loss or duplicate work.

The desired later flow is **Send download to → Home server → Pato**: **Home server** is the instance and **Pato** is its profile. The selected instance determines the output filesystem. A server-accessible link can be fetched directly by that server; browser-bound authentication, POST-generated downloads, and `blob:` content require separate handling. Universal interception or zero client-side transfer is not promised for every browser download.

The extension depends on stable intake contracts, not on completion of every graph-editor feature. Both tracks remain deferred; detailed sequencing will be chosen after the local foundation is qualified. They do not imply a multi-user service, cloud account requirement, distributed transfer swarm, or exposure of the daemon's private socket.

### Cross-cutting acceptance

Schedulers must run outside the browser tab and define behavior after sleep, downtime, retries, and duplicate triggers. Rules must distinguish immediately known URL metadata from response metadata that may arrive later or remain unknown. History must distinguish an accepted request from completed publication; a later read cannot resolve every lost acknowledgment. Preserve the existing rule that retrying an unknown mutation is not automatic.

Profile aggregate limits must be enforced across jobs rather than inferred from identical per-job caps. File actions must respect whether a file is local or on a hosted instance. Portable definitions must keep credentials and browser state separate by default, map machine-specific paths explicitly, and avoid treating active queue/manifest files as interchangeable configuration exports. Source tracking must preserve the current secret-free views and terminal source-clearing policy unless a separate retention contract is deliberately adopted.

The [W1 contract](local-web.md) settles the first local API, persistence, polling, and build/lifetime choices. Use [open decisions](decisions.md#web-application-direction) for richer telemetry, automation, library/history, and extension/hosting contracts. These specifications do not install dependencies or implement commands, services, endpoints, storage, or runtime support. Keep the original design as historical context and preserve the existing transfer and storage guarantees.

### Research references

The approved direction was informed by [Vue Flow](https://vueflow.dev/guide/), [Vue application approaches](https://vuejs.org/guide/extras/ways-of-using-vue), [Go embedded assets](https://pkg.go.dev/embed#hdr-File_Systems), [Playwright downloads](https://playwright.dev/docs/downloads) and [tracing](https://playwright.dev/docs/trace-viewer), [Firefox native messaging](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_messaging), and [scheduling policies](https://docs.temporal.io/schedule). These are capability references, not evidence that Yoink implements the planned features. Recheck exact versions and integration behavior when implementing each phase.

## Other directions

| Direction | Current disposition |
|---|---|
| Custom completion hooks and arbitrary JavaScript/Lua/WASM execution | Unscheduled; separate execution contract from typed browser workflows and event triggers |
| Other protocols, torrents, Metalink, distributed transfers, LAN swarm, and peer discovery | Unscheduled; not implied by a self-hosted web instance |
| Adaptive chunking and advanced fairness | Revisit with measurements after concrete scheduling needs are defined |
| General plugin/workflow DSL | No selected implementation; avoid making a framework a prerequisite for the app |
| Protected-content bypassing | Out of scope |
| Voice notifications, mobile UI, and other speculative interfaces | Unscheduled |
| User checksum flag, cleanup command, and short URL aliases | Separate interface decisions when justified |

Track unresolved contracts in [open decisions](decisions.md). Keep each implemented slice focused, validated, and documented in root STATUS before continuing.
