# Project status

Updated: October 23, 2026 (America/Sao_Paulo).

## Current state

**C1a is complete; C1b is in progress. The strict nested config codec passes focused tests, bounded fuzzing, and the full gate. Content tokens are next.** W1.1a's earlier version-1 app model/codec remains the tested implementation baseline and needs a version-2 ledger revision before W1.1b persistence.

No config loader/command, filesystem app store, HTTP, or frontend exists. Standalone/Linux daemon runtime is unchanged. Work stays local/no-push.

## Handoff

- `internal/jsoncheck/text.go` shares the existing app/queue UTF-8 and surrogate-pair guard without changing their schemas or errors. It validates Unicode, not the entire JSON grammar; callers must still enforce syntax, fields, types, and size bounds. The config codec reuses it.
- `internal/config/model.go` and `value.go` implement caller-supplied identity/state initialization, structural validation, a seven-key catalog, typed immutable values, sparse overrides, exact ID/name lookup, and deep copies. No IO exists. The codec adapts these flat keys to the required nested JSON format; model validation remains structural.
- `internal/config/resolve.go` adds explicit global/profile Scope, Get/List for explicit values, Resolve with ordered origins and normalized scalar settings, and Defaults for missing-config built-ins. Invalid scope/config returns no partial data. Unconfigured output remains distinct from empty text; resolutions keep their settings after later config edits. Per-request flags/filenames remain C3 work.
- `internal/config/edit.go` adds Set/Unset returning an owned candidate plus changed flag. Full structural validation brackets each edit; current identity, unrelated overrides, and source snapshots are preserved. Identical explicit values/unsetting absent values are no-ops with owned copies; setting an inherited value creates an override. Strict encoding applies the raw/canonical byte bounds separately.
- `internal/config/codec.go` and `decode.go` provide strict private encodeConfig/decodeConfig. Output has two-space indentation, fixed root/profile order, lexical setting order, preserved profile order, and one newline. Raw/canonical 256 KiB limits include expansion; preflight bounds model text. Required objects reject null, duplicate/unknown fields and noninteger counts are refused, Unicode is lossless, empty groups normalize to inheritance, and errors expose no partial state.
- [Configuration](docs/configuration.md) selects optional XDG `config.json`, sparse global/profile overrides, exact IDs/names, bounded strict JSON, and one authority shared by CLI/web. JSON replaces the illustrative TOML proposal and adds no dependency. Instance state binding is immutable; profile `storage.state_dir` means transfer checkpoint placement, not daemon relocation.
- `config init/list/get/set/unset`, effective values/origins, unset-to-inherit, unlimited values, command results, and exits are specified. Reads create nothing. Initialization is explicit. Managed writes use a short config lock and atomic durable saves; offline manual editing is supported without pretending advisory locks exclude arbitrary editors.
- Whole-file content tokens replace proposed numeric profile revisions, including stale-preview detection after inherited global/manual changes. [W1](docs/local-web.md) now specifies profiles only in config and a version-2 `app.json` ledger with captured profile ID/name, config token, and immutable settings. A separate `web.lock` owns the web lifetime; `app.lock` is transactional and serializes profiled dispatch/dismissal. Config-to-ledger is the only nested lock order.
- Current `internal/app/model.go`, `update.go`, and `codec.go` still implement the earlier version-1 schema with editable profiles/revisions. Reuse their strict validation/owned-copy patterns but revise the schema before loading/saving files. Encountered version-1 app files must be preserved/refused; no filesystem app store has shipped.
- Config CLI must precede frontend editing. Profile-aware Add must share binding-before-one-IPC admission; unprofiled jobs stay Unassigned. The daemon never reloads defaults for accepted requests. Transfer storage integration requires a separately versioned queue/IPC extension and compatibility proof.
- [Managed storage](docs/managed-storage.md) selects private checkpoint roots, same-filesystem partial staging, output-local hidden persistent locks/locators, and ordered durable cleanup. Existing sidecars resume in place; root edits do not retarget saved artifacts. Older standalone/daemon binaries cannot coordinate fresh hidden locks and are excluded from concurrent managed-layout use. Updated daemon v1 clients/imported jobs keep legacy semantics; queue/IPC v2 pins new storage preferences. All of this remains unimplemented.

## Validation

- `go test ./internal/config -count=1` passed model/resolution/edit tests and codec format, field/type/Unicode, exact size/count boundaries, canonical expansion, precision, ownership, and failure preservation.
- `go test ./internal/config -run '^$' -fuzz '^FuzzConfigCodec$' -fuzztime=5s -parallel=2` passed 138,412 executions; log: `/tmp/yoink-c1b-codec-fuzz.log`.
- `make check TMPDIR="$PWD/bin/t.mOAkr6"` passed formatting, vet, all ordinary/race tests, and build for the codec; log: `/tmp/yoink-c1b-codec-check.log`. Documentation links/examples and `git diff --check` passed.
- Shared Unicode extraction passed focused tests and its full gate; log: `/tmp/yoink-c1b-unicode-check.log`. Initial baseline sockets were sandbox-blocked, and two pre-staging packaging runs failed on the moved file until the Git index update was permitted. No product schema behavior changed in that refactor.

## Next steps

1. Finish C1b with validated Encode/Decode entry points that pair results with lowercase SHA-256 tokens of the exact returned/input bytes. Formatting-only edits must change tokens while retaining equivalent config. Failures return no bytes/config/token; add token and fuzz assertions, then the full gate. Keep IO/CLI separate.
2. Follow [the ordered slices](docs/managed-storage.md#implementation-order-and-acceptance): after C1b, C2a read-only discovery/loading; C2b atomic config transactions/init; C2c read then edit CLI commits. Keep runtime consumer availability explicit.
3. Revise the app ledger model/codec (W1.1a2), then implement W1.1b ownership/loading and W1.1c atomic transactions. Follow S10a–S10d for managed paths/records, filesystem behavior, queue migration, and IPC; split these responsibilities further as needed.
4. Add C3 request/profile adapters, including shared binding-before-Add, before W1 HTTP/assets/frontend and full acceptance. W2–W4 and Firefox/hosted L1/L2 remain later.

## Open decisions and blockers

No product decision blocks C1b. C09, Linux config discovery, and S10 placement/compatibility are selected; no runtime implementation or new platform qualification is claimed. Other-platform discovery, daemon-state migration, profile lifecycle CLI, system config, extra settings, richer budgets, library/history, scheduling, and Firefox/hosted integration remain deferred.
