# Project status

Updated: October 22, 2026 (America/Sao_Paulo).

## Current state

**C09/S10 contracts are selected, not implemented. Next: C1a, the pure config model/resolver.** W1.1a's earlier version-1 app model/codec remains the tested implementation baseline and needs a version-2 ledger revision before W1.1b persistence.

No config loader/command, filesystem app store, HTTP, or frontend exists. Standalone/Linux daemon runtime is unchanged. Work stays local/no-push.

## Handoff

- [Configuration](docs/configuration.md) selects optional XDG `config.json`, sparse global/profile overrides, exact IDs/names, bounded strict JSON, and one authority shared by CLI/web. JSON replaces the illustrative TOML proposal and adds no dependency. Instance state binding is immutable; profile `storage.state_dir` means transfer checkpoint placement, not daemon relocation.
- `config init/list/get/set/unset`, effective values/origins, unset-to-inherit, unlimited values, command results, and exits are specified. Reads create nothing. Initialization is explicit. Managed writes use a short config lock and atomic durable saves; offline manual editing is supported without pretending advisory locks exclude arbitrary editors.
- Whole-file content tokens replace proposed numeric profile revisions, including stale-preview detection after inherited global/manual changes. [W1](docs/local-web.md) now specifies profiles only in config and a version-2 `app.json` ledger with captured profile ID/name, config token, and immutable settings. A separate `web.lock` owns the web lifetime; `app.lock` is transactional and serializes profiled dispatch/dismissal. Config-to-ledger is the only nested lock order.
- Current `internal/app/model.go`, `update.go`, `codec.go`, and `json.go` still implement the earlier version-1 schema with editable profiles/revisions. Reuse their strict validation/owned-copy patterns but revise the schema before loading/saving files. Encountered version-1 app files must be preserved/refused; no filesystem app store has shipped.
- Config CLI must precede frontend editing. Profile-aware Add must share binding-before-one-IPC admission; unprofiled jobs stay Unassigned. The daemon never reloads defaults for accepted requests. Transfer storage integration requires a separately versioned queue/IPC extension and compatibility proof.
- [Managed storage](docs/managed-storage.md) selects private checkpoint roots, same-filesystem partial staging, output-local hidden persistent locks/locators, and ordered durable cleanup. Existing sidecars resume in place; root edits do not retarget saved artifacts. Older standalone/daemon binaries cannot coordinate fresh hidden locks and are excluded from concurrent managed-layout use. Updated daemon v1 clients/imported jobs keep legacy semantics; queue/IPC v2 pins new storage preferences. All of this remains unimplemented.

## Validation

- Documentation-only contract update: `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed (26 Markdown files, 444 local links, 8 JSON and 37 shell examples), including the new storage contract; `git diff --check` passed. Go/browser tests were not rerun because runtime code is unchanged.
- Earlier October 20 implementation evidence: `go test ./internal/app -count=1`, bounded `FuzzSnapshotCodec` (197,514 executions), and `make check` passed for the model/update/codec commits. Logs: `/tmp/yoink-w1-model-check.log`, `/tmp/yoink-w1-update-check.log`, `/tmp/yoink-w1-codec-check.log`, `/tmp/yoink-w1-codec-fuzz.log`. These validate version 1, not the selected new config/ledger contracts.

## Next steps

1. Implement C1a: pure `internal/config` model, seven-key catalog, exact profile lookup, scoped set/unset, inheritance/origins, and effective-setting resolution. Inputs supply IDs/state path; no filesystem, codec, CLI, daemon, or transfer changes in this slice. Cover omission/unlimited, bounds, selectors, inherited globals, owned copies, and accepted-setting independence; run focused checks and `make check`.
2. Follow [the ordered slices](docs/managed-storage.md#implementation-order-and-acceptance): C1b strict codec/tokens; C2a read-only discovery/loading; C2b atomic config transactions/init; C2c read then edit CLI commits. Keep runtime consumer availability explicit.
3. Revise the app ledger model/codec (W1.1a2), then implement W1.1b ownership/loading and W1.1c atomic transactions. Follow S10a–S10d for managed paths/records, filesystem behavior, queue migration, and IPC; split these responsibilities further as needed.
4. Add C3 request/profile adapters, including shared binding-before-Add, before W1 HTTP/assets/frontend and full acceptance. W2–W4 and Firefox/hosted L1/L2 remain later.

## Open decisions and blockers

No product decision blocks C1a. C09, Linux config discovery, and S10 placement/compatibility are selected; no runtime implementation or new platform qualification is claimed. Other-platform discovery, daemon-state migration, profile lifecycle CLI, system config, extra settings, richer budgets, library/history, scheduling, and Firefox/hosted integration remain deferred.
