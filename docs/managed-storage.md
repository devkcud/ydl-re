# Managed transfer storage

[Documentation index](README.md) · [Configuration](configuration.md) · [Current storage](resume-and-storage.md)

**Selected S10 contract, October 22, 2026; not implemented.** Existing binaries still use sibling partial/manifest/lock files. This contract defines the later Linux layout, its compatibility boundary, and the gates required before changing that default. It does not move any current files.

## Placement

Separate small durable checkpoint records from partial download bytes. `storage.state_dir` selects a checkpoint root, defaulting to `<daemon-state>/transfers`. `storage.staging_dir` is `destination` by default, or an explicit private directory on the same publication filesystem/mount as the output. Both may be overridden per profile; neither changes the daemon queue or app ledger location.

For output `/downloads/archive.zip`, open and retain the actual output-parent directory. Let `key` be lowercase SHA-256 of the exact UTF-8 basename `archive.zip`; let `id` be a newly generated 32-lowercase-hex transfer identifier. A fresh managed transfer uses:

```text
/downloads/.yoinkdl/<key>/lock
/downloads/.yoinkdl/<key>/active.json
/downloads/.yoinkdl/<key>/<id>.part
<storage.state_dir>/<id>/state.json
```

With an explicit staging root, only the partial path changes to `<storage.staging_dir>/<id>/data.part`. The output-local lock and locator never follow profile/state-root changes. One hidden `.yoinkdl` directory replaces per-output sibling bookkeeping for fresh managed transfers; it is not a promise of zero filesystem artifacts. Reserve `.yoinkdl` as an output basename and preserve legacy reserved names.

Create only owned `0700` directories and `0600` files. Custom roots must already exist and be private/owned; built-in roots may be created when an actual transfer needs them. Refuse unsafe parents, symlinks, special files, hard-linked mutable artifacts, changed canonical identities, and mismatched records without repairing them. Do not chmod existing foreign directories. Anchor operations to opened directory/file handles and revalidate canonical identity, following the existing storage integrity rules.

## Destination ownership and locators

Every new-version writer takes the persistent nonblocking `.yoinkdl/<key>/lock` before inspecting/selecting a layout or changing transfer artifacts, and holds it through publication/cleanup. The key depends on the basename in the opened output directory, not a profile ID, configuration root, or caller's path spelling. Aliases of the same parent therefore contend on the same lock. An alias may still fail the existing exact manifest-path check; mutual exclusion does not authorize rewriting saved paths. Never unlink/recreate the lock during cleanup.

`active.json` is a strict, bounded 16 KiB version-1 locator with exactly `version`, `id`, `output`, `record`, and `partial`. All paths are clean absolute UTF-8 strings bounded by 4096 bytes, and `record` ends in `<id>/state.json` under the selected checkpoint root. It contains no URL, token, or progress. Validate the hash/name relationship, selected output, owner/mode/link count, and matching record before trusting it. An invalid, dangling, foreign, or conflicting locator is an error, never permission to start over.

`state.json` is a strict 128 KiB version-1 managed record with exactly `version`, `id`, `output`, `partial`, and `checkpoint`. Its immutable identity/path fields must match the locator. `checkpoint` embeds the existing version-1 prefix or version-2 chunk manifest without changing either manifest's semantics or 64 KiB bound. It may be null only during incomplete setup; a restarted reader refuses that state rather than inferring resumable bytes. Raw/canonical bounds, exact fields/types, lossless Unicode, fingerprints, and checksums remain mandatory.

Read-only `inspect <output>` checks the hidden locator and legacy artifacts without creating locks/directories or contacting a service. Existing validated locators pin recovery paths even after profile changes, root changes, config deletion, or a different invoking profile. Never scan arbitrary state roots to guess a matching transfer. If an old root is unavailable, report that saved state is unavailable and preserve everything; do not create a fresh transfer at today's root.

Accepted request storage fields are frozen preferences for creating fresh artifacts. A validated existing checkpoint retains its own recorded locations; reuse reports those retained locations rather than claiming a move occurred. It must still pass the existing source/version/output/checksum checks. Moving a destination directory, retargeting a symlink, manually moving state, or changing a locator does not establish compatible recovery; automatic relocation is outside this contract.

## Setup, checkpoints, and publication

1. Acquire destination ownership and settle legacy/managed selection. Refuse conflicting state. Validate all roots and the staging/publication filesystem before network transfer. Open the destination parent and storage handles; allocate a fresh ID without replacing an existing directory.
2. Exclusively create the private partial and managed record, then sync their files/directories. Atomically publish and sync the locator before receiving body bytes. A crash during setup may leave incomplete owned state; preserve/refuse anything whose safe recovery is not established. Never adopt orphan temps or unrelated records.
3. Sync verified bytes before atomically saving checkpoint progress. Keep the existing sequential-prefix/chunk checks, remote validator checks, conservative truncation, and one-writer rules. Record/locator operations across directories are ordered durable steps, not a claimed multi-file atomic transaction.
4. Verify the complete file and persist a ready checkpoint before final publication. Retain existing no-overwrite hard-link publication and explicit standalone overwrite via rename, with same-filesystem guarantees. Cross-filesystem staging is refused; there is no automatic copy fallback. Matching device IDs alone are insufficient where mount boundaries prohibit publication; qualify the actual operation or refuse before transfer.
5. Sync the final directory, remove the partial name, and sync its directory. Only then remove and sync the active locator; remove the checkpoint record last and sync its directory. Keep the ready record available while the locator can still point to it. A crash after locator removal may leave an orphan record, which is preserved rather than reused as recovery authority. Permanent hidden lock directories remain.

Cleanup failures retain the verified publication outcome. Ready recovery can verify the final/partial identity and finish cleanup; file existence alone never proves success. Interrupted/failed transfers retain their locator, record, and partial. Cancel/forget/dismiss keep the current artifact-preservation behavior. Explicit cross-filesystem copying, orphan cleanup, and state migration tools are separate later work.

## Existing transfers and older executables

New-version writers acquire hidden destination ownership first. If a legacy lock exists, also acquire that exact persistent inode before proceeding, including for a new managed transfer after an earlier legacy completion. Legacy partial/manifest state is resumed in place using the old layout and validation; do not rename, convert, or delete it merely because configured roots changed. A lone legacy partial remains incompatible. Legacy and managed active records together are a conflict. Old lock files are retained after success; there is no safe blanket deletion rule.

A request marked legacy, including an imported queued job or a version-1 daemon Add, uses the old sidecars for a fresh transfer and takes both new and legacy locks. A managed request uses the hidden layout only when no legacy partial/manifest exists. This allows the new engine to serialize its managed and legacy requests across profiles, CLI processes, and the daemon.

Older standalone and daemon binaries do not know the hidden lock and cannot be promised safe concurrent use with a fresh managed transfer that has never had a legacy lock. Managed-layout qualification therefore covers updated writers; older transfer-owning executables must be stopped before cutover and must not run concurrently against those destinations. Do not claim simultaneous old/new transfer-engine support or weaken the persistent-lock rule to hide this limit. Existing legacy transfers and version-1 clients of the updated daemon remain supported through the rules above. Document this boundary before enabling the new default.

## Queue and IPC boundary

The daemon must receive immutable storage choices with the request, not load a profile or infer locations at dispatch. Introduce a version-2 queue snapshot/request and version-2 IPC Add before enabling configured storage for queued jobs. The new normalized request adds exactly `storage`, with `layout`, `state_dir`, and `staging_dir`: layout is `legacy` or `managed`; legacy roots are null, managed state root is absolute, and managed staging is `destination` or an absolute root. These same fields enter app submission settings. No raw config or profile authority is copied into the daemon.

Keep the current version-1 Add shape accepted by the updated daemon, translating it to legacy storage explicitly. New clients send version 2 for managed storage; an old daemon rejects it, and clients never downgrade/retry an uncertain mutation. Safe read/control operations may continue to support version 1. Retain current request-size, response-size, UID, and source-privacy limits. Root choices participate in the immutable managed request hash; use a distinct hash domain. Legacy requests retain the old hash calculation so retained version-1 IDs still deduplicate correctly.

Under exclusive daemon/queue ownership, validate an existing version-1 queue, map every request to legacy storage without changing ID/order/source/settings/outcome, and atomically commit version 2 before recovery/admission. Preserve the original canonical snapshot on pre-rename failure; uncertain commit stops startup. The older binary must refuse the version-2 snapshot rather than downgrade it. No automatic rollback or transfer-sidecar migration occurs. Capacity/size validation must run on the upgraded encoding before replacement; refusal leaves the old snapshot intact. Fresh queued requests pin new-artifact roots at admission, and pause/resume/restart never reads current config.

This versioned upgrade is a required separate implementation and qualification slice. Current queue/IPC code remains version 1, and current transfer.Request has no storage field. Do not advertise configured queue storage before all three layers preserve these values across normalization, admission, persistence, recovery, and execution.

## Implementation order and acceptance

The following order supersedes proceeding directly from the earlier W1.1a model to app filesystem persistence. Each row may need multiple focused commits; include its own tests/docs and STATUS update.

| Step | Responsibility and prerequisite |
|---|---|
| C1a | Pure config model, seven-key catalog, exact selectors, sparse mutations, inheritance/origins, owned copies, and resolved settings; no IO/codec/CLI work |
| C1b | Strict bounded config JSON codec, canonical encoding, content tokens, malformed input and fuzz coverage |
| C2a | Linux XDG discovery and private read-only config loading; missing-file reads create nothing |
| C2b | Config transaction ownership, atomic init/set/unset, current-file reload, stale-token and failed/uncertain-commit behavior |
| C2c | Config read commands and versioned text/JSON results, then init/edit CLI in a separate commit; no service startup |
| W1.1a2 | Pure version-2 app ledger/codec and immutable submission guards; remove editable profiles/revisions from runtime state |
| W1.1b | Linux ledger ownership/loading and separate web-lifetime ownership; refuse version-1/foreign/ambiguous state |
| W1.1c | Atomic ledger initialization/transactions and fail-stop recovery; shared config-before-ledger lock order |
| S10a | Pure managed locator/record codec and path planning; preserve embedded manifest v1/v2 meaning |
| S10b | Destination-wide ownership, legacy selection, pinned-locator discovery, and same-filesystem root checks |
| S10c | Managed setup/checkpoint/publication/cleanup and crash/fault preservation; qualify the engine before enabling the layout |
| S10d | Version-2 queue/request and IPC Add, owned v1 upgrade, hash compatibility, pinned settings across recovery; split persistence and IPC commits |
| C3a | Configured standalone/global queue request adapters, explicit profile selection, storage roots, and consistent default cutover; separate standalone/queue commits |
| C3b | Profiled CLI Add using shared ledger binding, no replay, lost-output/ack recovery, and concurrent web ownership tests |
| W1.2 onward | HTTP/read views, web/assets, frontend, shared profile editing, preview/admission/control UI, then full acceptance |

C2 configuration commands may persist the selected settings before consumers exist, but must identify this development boundary in help/results: stored defaults are not applied to today's download/add commands until C3. Do not silently claim a setting is active. Release/expose the integrated feature only after its dependent gates; frontend controls remain disabled until the corresponding API/runtime behavior exists.

Required fixtures include: two profiles/roots racing one output; output-parent aliases; legacy/managed conflicts; old lock preservation; corrupt/missing/foreign locators; configured root changes/deletion/unavailability; same-filesystem custom staging and cross-device refusal; crashes at each locator/record/publication/cleanup boundary; no-overwrite and explicit overwrite; sequential/chunk and ready recovery; v1 queue upgrade failures; v1 Add hash deduplication; v2 rejection without fallback; daemon restart with config changed; and profiled CLI/web concurrent admission/dismissal. Use owned temporary roots/local HTTP servers, then `make check`; actual mount/cross-device checks need an environment that supports them. Document native/runtime limits separately from compile evidence.
