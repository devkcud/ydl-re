# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.2.0-rc.1](https://github.com/devkcud/ydl-re/releases/tag/v0.2.0-rc.1) is published as a qualified Linux/amd64 prerelease; stable `0.1.0` remains Latest. Milestones A through D and v0.2 E1 through E6 are complete. Published tags, notes, and assets remain immutable; the named release's push exception is complete.

The first v0.3 slice is now specified in [Initial terminal interface](docs/tui.md): monitor existing Linux daemon jobs and use pause/resume/cancel/forget through current IPC. Implementation has not started. `yoinkdl tui`, configuration loading, profiles, history, and hooks remain unavailable. Future work stays local/no-push.

## Handoff

- [F1 through F4](docs/tui.md#implementation-order) orders pure model/rendering, bounded polling/controls, Linux terminal/CLI integration, and real daemon/PTY qualification. Start with F1; do not add a CLI command or terminal/IPC side effects to that step.
- Reuse `daemon.List`, `queue.JobView`, `daemon.Resolve`, and `daemon.Call`. The TUI owns local selection, confirmation, display, request scheduling, and terminal lifetime only. Queue storage, transfer contexts, source URLs, and publication remain daemon/engine responsibilities; no wire or storage migration is selected.
- C08 is settled for the first slice by bounded polling of existing safe snapshots. Later retry counters/subscriptions remain open. C09/S08 are deferred to configuration/profiles work and do not block F1–F4. No illustrative TOML schema or configuration-root layout was adopted.
- The contract specifies one in-flight request, stable full-ID selection, cancel/forget confirmation, no automatic mutation retry, explicit unknown-outcome notices, and refreshed views before further controls. UI checks do not strengthen the daemon's existing ID-based concurrency semantics. Terminal failure/quit/suspension must leave accepted jobs independent.
- Initial rendering uses ASCII labels and visible escapes for non-ASCII/control text, preserving original model values. This provides bounded terminal-cell layout with the standard library; existing CLI text behavior stays unchanged. The renderer and terminal limits, keys, exit behavior, cleanup, and acceptance cases are defined in the contract.
- Preserve prerelease source `34041c372519ac751bb9d6ec1ecda56184d13161`, tag object `31bebc5dfc543d318aa47b5ec5fb9c1f69657fa7`, and release ID `417837910`. [Published release evidence](docs/releases.md#v02-hosted-qualification-and-publication) owns hashes, hosted runs, public installation, and prior-release preservation. Local evidence remains in ignored `bin/v02-release.8Go8z2/` and `bin/e6-qualification.0Stkvu/`.

## Validation

- Cross-checked the planned views against `internal/queue/views.go` and `internal/daemon/replies.go`; control eligibility and slot retention against `internal/queue/lifecycle.go` and `controls.go`; single-attempt uncertainty/ID handling against the daemon client and queue CLI. Existing terminal/output ownership and Linux PTY fixtures were inspected for the implementation boundaries.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed the maintained Markdown files, local links/anchors, shell/JSON/TOML examples, fences, and whitespace, including the new untracked contract. `git diff --check` passed; the staged diff is checked before commit.
- No Go/runtime/build/workflow files or dependencies changed. `make check` and new runtime tests were not run for this documentation-only planning step. The TUI contract and acceptance matrix are not implementation or terminal-runtime evidence.
- Existing [hosted CI](https://github.com/devkcud/ydl-re/actions/runs/39338571399), [tagged packaging](https://github.com/devkcud/ydl-re/actions/runs/39338981764), and [release-report CI](https://github.com/devkcud/ydl-re/actions/runs/39340108218) passed before this plan. Linux/amd64 is the only qualified native target; foreign compilation and ordinary process-death fixtures do not establish other native or power-loss guarantees.

## Next steps

1. Implement F1 in a new `internal/tui` package: pure model/rendering over safe snapshots, selection by full ID, confirmation invalidation, honest unknown/decreasing counters, and bounded ASCII-escaped layout. Cover empty/full queues, all states, reordered/removed jobs, colliding display prefixes, malicious/long text, and viewport/frame limits with deterministic unit fixtures. Run focused tests, `make check`, and refresh this handoff before committing locally.
2. Implement F2's injected polling/control loop with fake time, one in-flight call, no mutation backlog/retry, safe failures, explicit uncertainty, and joined local cancellation.
3. Implement F3 Linux terminal lifetime/input and CLI integration, then F4 real daemon/PTY and CGO-disabled/six-target qualification. Do not claim the first TUI slice complete before those gates pass.
4. Specify configuration/profiles, richer history, and hooks separately after this first slice. Stable `0.2.0` promotion requires a separate explicit release request and readiness review; never change existing published assets or push unrelated future work.

## Open decisions and blockers

No known product decision blocks F1. Richer C08 event fields/subscriptions, C09 configuration semantics, and S08 configuration discovery remain open for later slices. Unicode-width rendering beyond the initial escaped display needs a separate concrete dependency/qualification decision. Native platforms beyond Linux/amd64, real power-loss durability, signing, and package-manager distribution remain separate work.
