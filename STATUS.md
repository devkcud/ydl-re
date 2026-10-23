# Project status

Updated: October 23, 2026 (America/Sao_Paulo).

## Current state

**C1a and C1b are complete:** the pure config model/resolver/edits, strict JSON codec, and validated content tokens pass focused tests, bounded fuzzing, and full gates. **Next: C2a read-only Linux XDG discovery/loading.**

No config loader/command, filesystem app store, HTTP, or frontend exists. Standalone/Linux daemon behavior is unchanged. Work stays local/no-push.

## Handoff

- `internal/config/model.go`, `value.go`, `resolve.go`, and `edit.go` implement caller-supplied identity/state initialization, typed sparse overrides, exact profile lookup, structural validation, owned copies, inherited values/origins, and pure Set/Unset. No-op results own their maps; setting an inherited value creates an explicit override. Per-request flags/filenames remain C3 work.
- `codec.go`/`decode.go` map flat Overrides to strict nested version-1 JSON. Root/profile fields have fixed order, group/setting keys sort lexically, profile order is preserved, and output uses two-space indentation plus one newline. Empty groups normalize to inheritance. Raw/canonical 256 KiB bounds include expansion; a model-text budget bounds encoding work. Required fields/nulls/types/integer syntax/Unicode and collection/model rules are checked; invalid input returns no partial result or parser diagnostics. Config.Validate remains structural.
- `token.go` exposes Encode(Config) -> bytes/token/error and Decode(bytes) -> config/token/error. Tokens are lowercase SHA-256 of exact validated returned/input bytes. Formatting-only changes invalidate read tokens. Future IO must use these entry points, compare selected-state binding separately, and retain original bytes/tokens for no-op edits instead of rewriting equivalent noncanonical JSON.
- `internal/jsoncheck` shares the pre-existing app/queue Unicode guard; those schemas/errors are unchanged. It checks Unicode rather than full JSON syntax. Config, app, and queue codecs retain their own schema validation.
- [Configuration](docs/configuration.md) selects one optional XDG config.json, shared CLI/web transactions, immutable instance/state binding, and scoped init/list/get/set/unset. Profile storage.state_dir means transfer checkpoints, not daemon relocation. C2 reads must remain independent of daemon startup and output/storage-directory availability.
- `internal/app` still implements the earlier pure version-1 profile/revision model. Before app persistence, revise it to the [version-2 submission ledger](docs/local-web.md#private-application-metadata): frozen profile ID/name, config token, and settings; no editable profiles. Preserve encountered v1 files. Config-before-ledger is the selected lock order; web lifetime has a separate lock.
- [Managed storage](docs/managed-storage.md) remains unimplemented: hidden persistent destination locks/locators, same-filesystem staging, retained legacy partials, and version-2 queue/IPC storage preferences. Older transfer engines cannot coordinate fresh hidden locks. Follow its compatibility/qualification order before enabling storage or request adapters.

## Validation

- `go test ./internal/config -count=1` passed model/resolution/edit and codec/token cases: malformed fields/types/Unicode, exact size/count limits, canonical expansion, precision, owned results, exact-byte tokens, and no partial failures. Source is formatted with gofmt.
- `go test ./internal/config -run '^$' -fuzz '^FuzzConfigCodec$' -fuzztime=5s -parallel=2` passed 263,263 executions through public Encode/Decode; `/tmp/yoink-c1b-token-fuzz.log`. The earlier private-codec run passed 138,412 executions; `/tmp/yoink-c1b-codec-fuzz.log`.
- `make check TMPDIR="$PWD/bin/t.mOAkr6"` passed formatting, vet, all ordinary/race tests, and build for each focused change. Logs: `/tmp/yoink-c1b-unicode-check.log`, `/tmp/yoink-c1b-codec-check.log`, `/tmp/yoink-c1b-token-check.log`.
- Initial baseline sockets/index writes hit sandbox limits; two pre-staging packaging runs also saw the moved file in the old index. Permitted reruns after staging passed; schemas were not relaxed. Packaging fixtures use Git's file list, so stage moves before their gate.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` and `git diff --check` passed, including new-file links/examples/whitespace. Filesystem-loader/CLI/frontend behavior is not claimed by these pure-code tests.

## Next steps

1. Implement C2a: read-only Linux XDG config discovery and private bounded loading. Reuse/extract state-only path resolution without requiring a usable daemon socket; call Decode and compare the decoded state binding with the selected state path. Distinguish absent config from invalid/unsafe/foreign files.
2. Reads must create no directories, locks, temps, or services and preserve every artifact. Check config owner/mode/link count/identity and bound reads before decoding; historical profile output/storage directories need not exist. Test temporary-root success/refusal/missing cases and run make check. Keep C2b locking/init/atomic mutations and C2c CLI adapters separate.
3. Follow [the ordered slices](docs/managed-storage.md#implementation-order-and-acceptance) for C2b/C2c, then the version-2 app ledger model, W1.1b ownership/loading and W1.1c transactions, S10 managed storage/queue/IPC, and C3 request adapters.
4. Keep W1 HTTP/assets/frontend and full acceptance after their prerequisites. W2–W4 and Firefox/hosted L1/L2 remain later.

## Open decisions and blockers

No product decision blocks C2a. Other-platform discovery, daemon-state migration, profile lifecycle CLI, system config, extra settings, richer budgets, library/history, scheduling, and Firefox/hosted integration remain deferred. No new platform or filesystem-runtime qualification is claimed.
