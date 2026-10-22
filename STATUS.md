# Project status

Updated: October 22, 2026 (America/Sao_Paulo).

## Current state

**C09 configuration/CLI ownership is specified, not implemented.** The next design task is S10 transfer placement and compatibility; then implement the pure config model/resolver. W1.1a's earlier version-1 app model/codec remains the tested implementation baseline and needs a version-2 ledger revision before W1.1b persistence.

No config loader/command, filesystem app store, HTTP, or frontend exists. Standalone/Linux daemon runtime is unchanged. Work stays local/no-push.

## Handoff

- [Configuration](docs/configuration.md) selects optional XDG `config.json`, sparse global/profile overrides, exact IDs/names, bounded strict JSON, and one authority shared by CLI/web. JSON replaces the illustrative TOML proposal and adds no dependency. Instance state binding is immutable; profile `storage.state_dir` means transfer checkpoint placement, not daemon relocation.
- `config init/list/get/set/unset`, effective values/origins, unset-to-inherit, unlimited values, command results, and exits are specified. Reads create nothing. Initialization is explicit. Managed writes use a short config lock and atomic durable saves; offline manual editing is supported without pretending advisory locks exclude arbitrary editors.
- Whole-file content tokens replace proposed numeric profile revisions, including stale-preview detection after inherited global/manual changes. [W1](docs/local-web.md) now specifies profiles only in config and a version-2 `app.json` ledger with captured profile ID/name, config token, and immutable settings. A separate `web.lock` owns the web lifetime; `app.lock` is transactional and serializes profiled dispatch/dismissal. Config-to-ledger is the only nested lock order.
- Current `internal/app/model.go`, `update.go`, `codec.go`, and `json.go` still implement the earlier version-1 schema with editable profiles/revisions. Reuse their strict validation/owned-copy patterns but revise the schema before loading/saving files. Encountered version-1 app files must be preserved/refused; no filesystem app store has shipped.
- Config CLI must precede frontend editing. Profile-aware Add must share binding-before-one-IPC admission; unprofiled jobs stay Unassigned. The daemon never reloads defaults for accepted requests. Transfer storage integration requires a separately versioned queue/IPC extension and compatibility proof.
- S10 still needs the final hidden locator/lock layout, same-filesystem staging policy, root-change recovery, cleanup ordering, and coexistence limits with older standalone binaries. Existing sidecars remain implemented behavior until that work is qualified.

## Validation

- Documentation-only contract update: `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed local links/anchors and JSON/shell examples; `git diff --check` passed. Go/browser tests were not rerun because runtime code is unchanged.
- Earlier October 20 implementation evidence: `go test ./internal/app -count=1`, bounded `FuzzSnapshotCodec` (197,514 executions), and `make check` passed for the model/update/codec commits. Logs: `/tmp/yoink-w1-model-check.log`, `/tmp/yoink-w1-update-check.log`, `/tmp/yoink-w1-codec-check.log`, `/tmp/yoink-w1-codec-fuzz.log`. These validate version 1, not the selected new config/ledger contracts.

## Next steps

1. Finish S10's transfer layout, destination-wide ownership, old-job discovery, legacy interoperability, and queue/IPC compatibility contract; record the complete implementation dependency order.
2. Implement C1a: pure `internal/config` model, seven-key catalog, exact profile lookup, scoped set/unset, inheritance/origins, and effective-setting resolution. No filesystem, codec, CLI, daemon, or transfer changes in this slice. Cover omission/unlimited, bounds, selectors, inherited globals, owned copies, and accepted-setting independence; run focused checks and `make check`.
3. Add the strict config codec/token slice, revise the app ledger model/codec, then separate discovery/loading and atomic config/app persistence. Build CLI config before frontend editing; integrate consumers and storage only after their prerequisites.
4. Resume W1 HTTP/assets/frontend, admission/controls, and full acceptance in the reconciled order. W2–W4 and Firefox/hosted L1/L2 remain later.

## Open decisions and blockers

C09 and Linux config discovery are resolved by the selected contract; no runtime implementation is claimed. S10 placement/compatibility is the remaining design checkpoint. Other-platform discovery, daemon-state migration, profile lifecycle CLI, system config, extra settings, richer budgets, library/history, scheduling, and Firefox/hosted integration remain deferred.
