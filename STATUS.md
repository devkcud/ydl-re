# Project status

Updated: October 19, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.2.0-rc.1](https://github.com/devkcud/ydl-re/releases/tag/v0.2.0-rc.1) is published as a qualified Linux/amd64 prerelease; stable `0.1.0` remains Latest. Milestones A through D and v0.2 E1 through E6 are complete. Published tags, notes, and assets remain immutable; the named release's push exception is complete.

The owner has dropped the TUI entirely. Its draft contract and implementation sequence were removed before any runtime code was written. The next product direction and v0.3 scope await the owner's new plans. Profiles, configuration loading, inspection/history extensions, and hooks remain unimplemented proposals, not selected next steps. Future work stays local/no-push.

## Handoff

- Do not resume the canceled terminal-interface work or recreate its package/command. The contract, implementation sequence, planned validation, navigation links, and architecture references have been removed from maintained documentation.
- [Roadmap scope](docs/roadmap.md#v03--scope-pending) and the [scope reset](docs/decisions.md#v03-scope-reset) record the owner's decision. The original `yoinkdl-design.md` remains a historical proposal; its TUI references are superseded by the current roadmap.
- TUI-specific polling, rendering, input, terminal-lifetime, and confirmation policies are withdrawn. C08 remains open for any future consumer; existing daemon/queue CLI behavior and safe polling contracts remain authoritative. C09/S08 are still unresolved for any later configuration/profiles work.
- No replacement front end, protocol, feature, or implementation order has been selected. Obtain the new product direction before assigning work to v0.3; do not automatically substitute profiles, history, or hooks.
- Preserve prerelease source `34041c372519ac751bb9d6ec1ecda56184d13161`, tag object `31bebc5dfc543d318aa47b5ec5fb9c1f69657fa7`, and release ID `417837910`. [Published release evidence](docs/releases.md#v02-hosted-qualification-and-publication) owns hashes, hosted runs, public installation, and prior-release preservation. Local evidence remains in ignored `bin/v02-release.8Go8z2/` and `bin/e6-qualification.0Stkvu/`.

## Validation

- This is a documentation-only scope change. No Go/runtime/build/workflow files, dependencies, CLI progress handling, or published artifacts changed. No TUI runtime implementation existed to remove.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed 24 maintained Markdown files, 387 local links/anchors, 36 shell blocks, seven JSON examples, one TOML example, fences, and whitespace. `git diff --check` passed. The reference audit found no active command/package/implementation-plan references; remaining TUI mentions record its cancellation or identify the historical proposal.
- `make check` and new product tests are unnecessary for this documentation-only removal. Existing [hosted CI](https://github.com/devkcud/ydl-re/actions/runs/39338571399), [tagged packaging](https://github.com/devkcud/ydl-re/actions/runs/39338981764), and [release-report CI](https://github.com/devkcud/ydl-re/actions/runs/39340108218) passed before this change. Linux/amd64 remains the only qualified native target.

## Next steps

1. Obtain the owner's new direction for Yoink and record the intended workflow, scope, and acceptance conditions. The TUI is excluded; no replacement is implied.
2. Once that direction is known, define the next implementation slice and dependency order, resolving only the configuration, API, storage, or execution decisions it actually needs.
3. Preserve all published tags/notes/assets. Stable `0.2.0` promotion needs a separate explicit release request and readiness review; never push unrelated future work.

## Open decisions and blockers

The next product direction is the missing input for new implementation; the requested TUI removal is complete. C08/C09/S08 remain open until a selected feature needs them. Native platforms beyond Linux/amd64, real power-loss durability, signing, and package-manager distribution remain separate work.
