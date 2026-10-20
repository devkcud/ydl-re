# Project status

Updated: October 20, 2026 (America/Sao_Paulo).

## Current state

The owner approved a local-first web application around the existing Go download engine and daemon. Vue 3 + TypeScript is the frontend direction, and **Vue Flow is selected** for the Visual Grabber editor. The roadmap now tracks feature-building phases; version targets and release preparation are outside the current work queue.

The web app, profiles, library/history, rules, schedules, browser workflows, extension, and hosted targeting are planned, not implemented. The TUI remains canceled. Existing standalone/daemon functionality is unchanged; future implementation stays local/no-push.

## Handoff

- [Product roadmap](docs/roadmap.md) owns the accepted capabilities and order. W1 starts with a local downloads/queue app and basic profile defaults; W2 adds durable library/activity, rules, and richer management; W3 adds schedules and a small browser/workflow runner; W4 adds fuller Vue Flow authoring/debugging.
- Browser integration and hosted operation are explicitly later tracks L1/L2, not W1 prerequisites. Preserve the intended actions **Send this link to Yoink** and **Send download to → Home server → Pato**. Home server is an instance/execution host; Pato is a profile/workspace within it. No separate user-account model is implied.
- [Planned architecture](docs/architecture.md#planned-web-application) keeps the daemon authoritative for transfers, checkpoints, and final publication. An optional Go application API serves the frontend and owns application metadata; an optional Playwright worker handles browser automation. Vue Flow is the editor choice, not a durable workflow executor.
- Keep instance, profile, workflow version, run, job, and library-file identities separate. Freeze effective settings at submission. Current job requests remain immutable; priorities/reordering, aggregate profile limits, richer telemetry, and permanent history need explicit backend contracts rather than UI-only behavior.
- [Open decisions](docs/decisions.md#web-application-direction) covers C08–C13 and S08/S09: telemetry, configuration, local API/lifetime/authentication, profiles/job controls, rules/automation, later extension/hosting, and metadata/library/export schemas. No database schema, HTTP endpoint, dependency pin, command, or service was implemented by this plan.
- Evergreen [core integrity criteria](docs/testing.md#core-integrity-acceptance) and the [completed daemon foundation](docs/daemon-and-queue.md#delivery-and-validation) now live outside the feature roadmap. Release history remains in its [existing guide](docs/releases.md); preserve published tags, notes, and assets without making release work a prerequisite for building.

## Validation

- This is documentation-only planning. No Go/runtime/build/workflow files, dependencies, original design, or published assets changed; no implementation or runtime qualification is claimed for the planned app.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 401 local links/anchors, 36 shell blocks, seven JSON examples, one TOML example, fences, and whitespace. `git diff --check` passed; the staged diff is reviewed before commit.
- The roadmap audit passed all ten requested capability areas, the selected Vue Flow choice, both send-to examples, W1–W4/L1–L2 ordering, explicit later integration, and absence of software-version/release targets or stale TUI/old roadmap anchors. The source research informed direction only; exact dependency/API behavior still needs qualification when implementing.
- `make check` and new product tests were not run for this documentation-only change. Existing native qualification remains Linux/amd64; building a web UI does not qualify other daemon/storage platforms.

## Next steps

1. Define the concrete W1 local web contract: the first downloads/queue and profile workflow, safe view/control API, process and frontend-asset build/lifetime boundaries, local authentication/origin handling, profile/default ownership, and treatment of CLI-created jobs. Record focused acceptance/failure cases using the existing daemon APIs before implementation.
2. Implement W1 in dependency-ordered, runnable local slices with relevant frontend checks, engine/IPC regressions, and `make check` for Go changes. Preserve independent standalone use and refresh this handoff after each implementation step.
3. Follow W2–W4 after their prerequisites, selecting representative browser-download journeys early enough to qualify the handoff before the full visual editor. Keep extension/server selection in later tracks L1/L2.

## Open decisions and blockers

Product direction and the Vue Flow choice are settled. W1's exact API, profile/persistence, lifecycle, and build contracts remain to be defined. Browser credential/download handoffs, durable scheduling, rules, catalog identity, and later hosted transports require their own subsequent contracts. No release-preparation task blocks the current build roadmap.
