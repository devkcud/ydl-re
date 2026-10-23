# Project status

Updated: October 22, 2026 (America/Sao_Paulo).

## Current state

**C1a is in progress: the pure config model, typed key catalog, exact profile lookup, and owned copies are implemented. Next: inherited resolution, then scoped edits.** W1.1a's earlier version-1 app model/codec remains the tested implementation baseline and needs a version-2 ledger revision before W1.1b persistence.

No config loader/command, filesystem app store, HTTP, or frontend exists. Standalone/Linux daemon runtime is unchanged. Work stays local/no-push.

## Handoff

- `internal/config/model.go` and `value.go` implement caller-supplied identity/state initialization, structural validation, a seven-key catalog, typed immutable values, sparse overrides, exact ID/name lookup, and deep copies. No IO or codec exists. The future codec must adapt flat override keys to nested JSON and enforce encoded-size/field/null rules.
- [Configuration](docs/configuration.md) selects optional XDG `config.json`, sparse global/profile overrides, exact IDs/names, bounded strict JSON, and one authority shared by CLI/web. JSON replaces the illustrative TOML proposal and adds no dependency. Instance state binding is immutable; profile `storage.state_dir` means transfer checkpoint placement, not daemon relocation.
- `config init/list/get/set/unset`, effective values/origins, unset-to-inherit, unlimited values, command results, and exits are specified. Reads create nothing. Initialization is explicit. Managed writes use a short config lock and atomic durable saves; offline manual editing is supported without pretending advisory locks exclude arbitrary editors.
- Whole-file content tokens replace proposed numeric profile revisions, including stale-preview detection after inherited global/manual changes. [W1](docs/local-web.md) now specifies profiles only in config and a version-2 `app.json` ledger with captured profile ID/name, config token, and immutable settings. A separate `web.lock` owns the web lifetime; `app.lock` is transactional and serializes profiled dispatch/dismissal. Config-to-ledger is the only nested lock order.
- Current `internal/app/model.go`, `update.go`, `codec.go`, and `json.go` still implement the earlier version-1 schema with editable profiles/revisions. Reuse their strict validation/owned-copy patterns but revise the schema before loading/saving files. Encountered version-1 app files must be preserved/refused; no filesystem app store has shipped.
- Config CLI must precede frontend editing. Profile-aware Add must share binding-before-one-IPC admission; unprofiled jobs stay Unassigned. The daemon never reloads defaults for accepted requests. Transfer storage integration requires a separately versioned queue/IPC extension and compatibility proof.
- [Managed storage](docs/managed-storage.md) selects private checkpoint roots, same-filesystem partial staging, output-local hidden persistent locks/locators, and ordered durable cleanup. Existing sidecars resume in place; root edits do not retarget saved artifacts. Older standalone/daemon binaries cannot coordinate fresh hidden locks and are excluded from concurrent managed-layout use. Updated daemon v1 clients/imported jobs keep legacy semantics; queue/IPC v2 pins new storage preferences. All of this remains unimplemented.

## Validation

- `go test ./internal/config -count=1` passed constructor purity, identity/name/path/collection bounds, typed setting grammar, exact lookup, and copy independence. Source is formatted with `gofmt`.
- `TMPDIR="$PWD/bin/c1.kfIuuA" make check` passed formatting, vet, ordinary/race tests, and build with permitted cache/socket access; log: `/tmp/yoink-c1a-model-check.log`. The first sandbox attempt stopped at a read-only Go cache before product tests.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` and `git diff --check` passed, including new-file links and whitespace. Codec, filesystem, CLI, and browser tests are not applicable to this pure slice.

## Next steps

1. Complete C1a with explicit global/profile scope, raw setting reads, inherited resolution/origins and normalized settings, then owned scoped set/unset in a separate focused commit. Preserve explicit zero/unlimited, reject invalid scopes, and prove config changes cannot alter earlier resolutions. Run focused checks and `make check` for each change; keep IO/codec/CLI/daemon/transfer integration separate.
2. Follow [the ordered slices](docs/managed-storage.md#implementation-order-and-acceptance): C1b strict codec/tokens; C2a read-only discovery/loading; C2b atomic config transactions/init; C2c read then edit CLI commits. Keep runtime consumer availability explicit.
3. Revise the app ledger model/codec (W1.1a2), then implement W1.1b ownership/loading and W1.1c atomic transactions. Follow S10a–S10d for managed paths/records, filesystem behavior, queue migration, and IPC; split these responsibilities further as needed.
4. Add C3 request/profile adapters, including shared binding-before-Add, before W1 HTTP/assets/frontend and full acceptance. W2–W4 and Firefox/hosted L1/L2 remain later.

## Open decisions and blockers

No product decision blocks C1a. C09, Linux config discovery, and S10 placement/compatibility are selected; no runtime implementation or new platform qualification is claimed. Other-platform discovery, daemon-state migration, profile lifecycle CLI, system config, extra settings, richer budgets, library/history, scheduling, and Firefox/hosted integration remain deferred.
