# Configuration

[Documentation index](README.md)

**C1a model/validation and inherited resolution are implemented; scoped edits are next.** The current binary still uses built-in defaults and flags. It has no configuration loader or configuration commands. The selected configuration/state split below requires an app model revision before filesystem persistence.

## Current implementation

[internal/config/model.go](../internal/config/model.go) provides `New`, structural `Validate`, owned `Clone`, and exact owned `Profile` lookup. IDs and the state path are supplied by the caller. It initializes Default with no overrides and performs no filesystem, randomness, or daemon work. Validation covers profile/name uniqueness, selector ambiguity, Unicode, clean absolute paths, and the derived built-in checkpoint-path bound.

[value.go](../internal/config/value.go) defines the seven keys, immutable typed `Int`/`Text` values, and sparse `Overrides`. Absent keys mean inherit; explicit zero retries and `unlimited` stay distinct. Validation reuses engine bounds and rate parsing plus the CLI's duration grammar. Copies own profile slices and all override maps. Errors are fixed and expose no invalid value.

[resolve.go](../internal/config/resolve.go) implements explicit `Scope`, raw `Get`/`List`, and `Resolve` with lexical entries and `builtin`/`global`/`profile` origins. `Defaults` resolves built-ins from a supplied state path without a config file. Resolutions own their entries and contain normalized scalar `Settings`, including exact int64 nanoseconds/bytes-per-second. An unconfigured output has a zero `Value` in the view and an empty `Settings.OutputDir`; it is not an accepted explicit empty path. Later edits cannot mutate an earlier resolution. Per-invocation flags and output filenames remain request-adapter work.

This is an in-memory model, not the JSON representation: overrides are keyed by full setting name. The future codec must map them to the declared nested objects, reject missing/null required fields, and enforce raw/canonical 256 KiB bounds. Filesystem existence/ownership, scoped edits, tokens, IO, and CLI adapters are not implemented yet.

## Current defaults

The engine uses four connections, three additional retries, a `30s` network inactivity timeout, no rate cap, automatic compatible resume, and no overwrite. `--resume` requires an existing checkpoint. Existing standalone commands do not read configuration files. Transfer sidecars and daemon policy still follow the [current storage](resume-and-storage.md) and [daemon contract](daemon-and-queue.md).

## Configuration and storage design checkpoint

The checkpoint selects one optional user configuration file shared by CLI and web editing, sparse global/profile overrides, immutable settings captured for each accepted request, and separate recovery state. JSON is selected instead of the earlier illustrative TOML proposal: the existing strict JSON approach and standard library are sufficient, and CLI commands provide everyday editing. There is no parser dependency, second editable profile store, or general plugin/settings framework.

Instance identity and daemon locations belong to the instance. A profile selects output, transfer defaults, and transfer metadata/staging locations inside that instance; selecting it never switches or restarts a daemon. Download state means checkpoint metadata and partial bytes, not the daemon's queue or the app's submission ledger. File placement, root-change recovery, and legacy compatibility are selected in [managed transfer storage](managed-storage.md); that implementation remains separate.

## Paths and identity

Use `$XDG_CONFIG_HOME/yoinkdl/config.json`, falling back to `$HOME/.config/yoinkdl/config.json`. Ignore an empty or relative XDG base and use the fallback; reject invalid UTF-8/NUL paths and require an absolute usable HOME when needed. Clean absolute paths once. Only this user file is loaded: no working-directory search, system-file merging, config-path flag, or setting-specific environment overrides. This follows the [XDG configuration/state separation](https://specifications.freedesktop.org/basedir/latest/).

Daemon state/runtime paths continue to come from `daemon.Resolve`, including its existing runtime fallback. Configuration records the selected daemon state path in a read-only identity header; mismatch is an error, not an implicit instance switch. `storage.state_dir` below changes only per-transfer checkpoint placement. A daemon-state-root migration is outside this contract.

Config commands and standalone downloads need only state-path resolution, not a usable IPC socket. Reuse/extract the pure state-root portion of daemon resolution; validate runtime/socket constraints only for operations that use the daemon endpoint.

`config init` explicitly creates a random instance ID and one random **Default** profile ID without an output directory. IDs are 32 lowercase hexadecimal characters. Initialization requires an absent config, no ambiguous config temps, and no existing app ledger bound to a missing config. It creates only its private config directory/files, never queue or transfer artifacts. An existing file is preserved and reported, even if valid; initialization is not reset.

A missing file allows unprofiled standalone/queue requests to use built-in defaults. Read-only config commands do not initialize it. Profile selection requires an initialized file. Web startup and profiled queue admission require the config and matching instance/state binding. Deleting or moving the config is not a state migration or a way to reset retained jobs.

## Configuration file

Version 1 has exactly `version`, `instance_id`, `state_dir`, `defaults`, and `profiles`. `defaults` and each profile's `settings` contain optional `download` and `storage` objects with the keys below. A profile has exactly `id`, `name`, and `settings`. Empty objects mean inherit. The following is a valid selected-format example, not a file read by today's binary:

```json
{
  "version": 1,
  "instance_id": "11111111111111111111111111111111",
  "state_dir": "/home/pato/.local/state/yoinkdl",
  "defaults": {
    "download": {"connections": 4, "retries": 3, "timeout": "30s", "limit": "unlimited"}
  },
  "profiles": [
    {"id": "22222222222222222222222222222222", "name": "Default", "settings": {}},
    {
      "id": "33333333333333333333333333333333",
      "name": "Pato",
      "settings": {
        "download": {"output_dir": "/home/pato/Downloads", "connections": 8},
        "storage": {"state_dir": "/home/pato/.local/state/yoink-work", "staging_dir": "destination"}
      }
    }
  ]
}
```

Require 1..32 profiles, unique IDs, and unique case-sensitive names. Names are trimmed, nonempty UTF-8 text of at most 80 bytes without control characters; names matching the 32-lowercase-hex ID grammar are reserved to avoid selector ambiguity. `--profile` resolves an exact ID or exact name, never a prefix or fuzzy match. Initial CLI scope covers settings on existing profiles; profile create/rename APIs follow later. Manual removal leaves existing submission records intact, with their captured names and IDs; it never reassigns them.

Bound raw and canonical configuration to 256 KiB. Reject unsupported versions, duplicate/unknown/missing required fields, null values, wrong types, lossy Unicode, invalid paths, and trailing JSON. Counts are JSON integers, not floats, exponents, strings, or booleans. Sparse settings preserve absence separately from explicit zero/unlimited. Canonical writes use two-space indentation, fixed field/key order, existing profile order, and one final newline. Comments are not part of this JSON format. No URLs, credentials, progress, submission records, or queue policy belong here.

## Settings and precedence

| Key | Value | Built-in value |
|---|---|---|
| `download.output_dir` | Clean absolute existing directory | Unconfigured; unprofiled CLI retains cwd/output-flag behavior |
| `download.connections` | Integer `1..32` | `4` |
| `download.retries` | Integer `0..10` | `3` |
| `download.timeout` | Positive duration using the existing CLI grammar, fitting signed int64 nanoseconds | `"30s"` |
| `download.limit` | Existing positive rate grammar or the exact string `"unlimited"` | `"unlimited"` |
| `storage.state_dir` | Clean absolute private checkpoint root | `<daemon-state>/transfers` |
| `storage.staging_dir` | `"destination"` or a clean absolute private staging root on the output filesystem | `"destination"` |

All seven settings support global and profile scope. A custom storage root must already exist, be owned by the effective user, and be private; the engine creates only its own children. Built-in private roots may be created on actual transfer use. Check configured directories on set/admission/use, not while decoding historical data; unavailable disks must leave config readable/editable. Staging/output filesystem compatibility is checked when both locations are known, before transfer bytes or incompatible partials are touched.

Resolve built-ins, then global overrides, then selected-profile overrides, then permitted invocation overrides. Persist only overrides in config; capture complete normalized settings for an accepted request. `unset` removes one override and restores inheritance, including a visibly unconfigured value when no fallback exists. Explicit `"unlimited"` overrides an inherited cap. Configuration does not add persistent resume or overwrite switches: safe resume remains automatic, `--resume` remains per-invocation require-resume, and overwrite remains an explicit standalone flag.

`download` and `add` will accept explicit `--profile <id-or-name>` once their adapters are qualified. Without it, use global defaults and retain unassigned queue attribution. An explicit CLI output flag retains its existing meaning; a selected profile supplies the default directory when no output flag is given. Web intake keeps its stricter selected-directory/basename policy. Profiled CLI Add must use the same binding-before-admission service as web Add; it cannot silently produce an Unassigned job. `inspect <output>` discovers pinned artifacts from the output, independent of today's profile selection.

The daemon never reloads profile defaults for queued/resumed jobs. Global/profile edits affect future requests only. Changes to inherited globals invalidate all observed config tokens, including previews, even for profiles whose effective values happened to stay equal.

## Shared editing and content tokens

Use a persistent private `config.lock` next to `config.json`. Each managed mutation acquires a nonblocking lock, reloads/validates the current file, applies only the requested change, validates the full result, verifies canonical artifact identity/content, and atomically writes/syncs/renames/syncs before acknowledging. Release ownership after that transaction; the web process must not monopolize this lock. Reads open bounded validated snapshots without creating directories or locks. Use `0700` for owned directories, `0600` for files, matching effective UID, no symlinks or hard-linked mutable files, and owned copies.

The concurrency token is lowercase SHA-256 of the exact loaded config bytes. It is not stored in the file and contains no source URL information. Web writes and previews carry `config_token`; compare it under config ownership before committing a change or freezing a submission. Any file change, including formatting or a global edit, makes an older token stale. This replaces the earlier proposed monotonic profile revision, which could not reliably detect manual edits. CLI set/unset applies to the latest file under the lock; it never saves an earlier whole-file copy. A no-op preserves the file and token.

Manual JSON editing is supported while managed writers are stopped. Advisory locks cannot exclude arbitrary editors; identity/content checks detect observed changes but cannot promise an atomic transaction against an editor ignoring the lock. There is no file watcher: each operation reloads/validates the file. A valid offline edit naturally changes the token; corrupt files are preserved and refused.

Pre-rename failure preserves the committed config. Post-rename sync failure reports `outcome_unknown`, stops that writer, and requires a fresh validated read before another write; never roll back by overwriting with an old snapshot or retry automatically. Output failure can follow a successful commit. A busy lock is a bounded error, not an unbounded wait.

## Planned CLI configuration

These are selected command contracts, not implemented commands:

```sh
yoinkdl config init
yoinkdl config list --global
yoinkdl config list --profile Pato --effective
yoinkdl config get download.connections --profile Pato --effective
yoinkdl config set download.connections 4 --global
yoinkdl config set download.connections 8 --profile Pato
yoinkdl config set download.limit unlimited --profile Pato
yoinkdl config unset download.connections --profile Pato
```

The Pato examples refer to the profile in the example file above; `config init` creates only Default. Creating/renaming another profile is available through offline file editing or the later web profile API until lifecycle CLI commands are added.

Except `init`, require exactly one of `--global` or `--profile`. Set/unset requires an existing initialized config; unknown profiles are errors. `list`/`get` without `--effective` show explicit values in that scope; effective reads show resolved values and origin (`builtin`, `global`, or `profile`). List keys in lexical order. A missing explicit value makes `get` report `unset` with exit 1; effective output-directory reads can return null/unconfigured. Unsetting an already absent known key is a successful no-op. CLI values are parsed by the key's type, not as arbitrary JSON fragments.

All operations accept `--json`. Emit one newline-terminated record with exactly `version: 1`, `command: "config"`, `operation`, `scope`, `profile_id`, `config_token`, `values`, `changed`, `exit_code`, and `error`. Scope is `global` or `profile` (`global` for init); profile ID is null for global. Token is null for an absent/unreadable config. Values is a sorted array of `{key, value, source}`; source is one of the origins above, value is the setting's integer/string or null for an unconfigured effective value. Successful writes return the affected effective entry and new token; init returns an empty values array. Failed operations return an empty values array and `{code, message}` with fixed safe text. Never echo invalid values or raw parser errors. Text output presents the same values, origins, and whether a write changed anything.

Exit 0 for success/no-op, 2 for invalid invocation/key/type/scope, 1 for unset/missing profile, config/ownership/conflict/IO/output/uncertain failure, and 130 after interruption cleanup. Missing or invalid configuration is never treated as an empty file to overwrite. Commands do not start services, mutate daemon policy, move files, or rewrite accepted requests. Build the shared config store and CLI before frontend editing; enable request consumers and storage keys only after their dependent behavior is qualified.

## Application metadata boundary

The planned `app.json` becomes a version-2 recovery ledger with exactly `version`, `instance_id`, `state_dir`, and `submissions`; it no longer stores editable profiles. Each submission captures `id`, `profile_id`, `profile_name`, `config_token`, and complete URL-free normalized `settings`: `output`, `connections`, `retries`, `timeout_ns`, `limit_bps`, `resume`, and the [pinned `storage` object](managed-storage.md#queue-and-ipc-boundary). There is no live-profile membership or revision-order requirement: accepted records survive profile removal/editing. Preserve the 512-record/4 MiB bounds, stable instance/state identity, owned copies, and immutable retained submissions.

The current version-1 pure model/codec must be revised explicitly before W1.1b; it has never had a production filesystem store. Reject encountered version-1 files without rewriting them; there is no silent migration. Config and ledger are separate commits, never a claimed two-file transaction. Under config ownership, freeze/validate a candidate and acquire the ledger transaction lock; durably save the binding before releasing config ownership and issuing the single daemon Add. Hold ledger ownership through the bounded Add reply so cross-process dismissal cannot race active dispatch. Other operations acquire config before ledger when both are needed, never the reverse. Ordinary config edits need no ledger write.

Use a short-lived `app.lock` per ledger transaction and a separate `web.lock` for the web process lifetime. Config commands work with both services stopped and with a live web process. Web/CLI profiled admission, forget, and dismiss share ledger validation and lock rules; busy operations fail explicitly. Read-only views load independent bounded snapshots and label unavailable reads rather than inventing empty state. No profile change retroactively edits a binding or authorizes replay.

## Daemon settings

The implemented `daemon start/set --parallel` policy remains durable queue state (`1..8`, default `1`); `daemon start --limit` remains a process-lifetime cap. They are not keys in the initial config schema. Saving profile settings cannot claim to apply a shared daemon cap or a live parallel change. Effective per-job limits continue to compose with the daemon budget. See [daemon settings](daemon-and-queue.md#queue-concurrency).

## Units

The rate parser distinguishes decimal and binary byte units explicitly:

| Unit | Bytes |
|---|---|
| `B` | 1 |
| `KB` | 1,000 |
| `MB` | 1,000,000 |
| `GB` | 1,000,000,000 |
| `KiB` | 1,024 |
| `MiB` | 1,048,576 |
| `GiB` | 1,073,741,824 |

For rates, `/s` means per second. Thus `5MB/s` is 5,000,000 bytes per second and `5MiB/s` is 5,242,880 bytes per second. Rates apply across all workers of the transfer.

`--limit` accepts ASCII decimal digits followed immediately by one of these units and `/s`. Values must be positive and convert to at most 9,223,372,036,854,775,807 bytes per second. Leading zeros are allowed. Signs, fractions, exponents, whitespace, missing units, different casing, and overflow are rejected. For example, write `1500KB/s` instead of `1.5MB/s`.

Omitting `--limit` means unlimited; explicit zero is invalid CLI rate syntax. The library's `Request.Limit` is a byte-per-second integer, with zero meaning unlimited and negative values rejected before file/network work. Arbitrary size expressions remain outside v0.1.

## Delivery and validation

The [complete dependency order](managed-storage.md#implementation-order-and-acceptance) is selected. The first implementation slice is a pure `internal/config` model, key catalog, scoped set/unset, profile lookup, inheritance with origins, and effective-setting resolution. It performs no filesystem or daemon work. Preserve the existing engine's normalization and validation responsibilities. A second focused slice adds the strict bounded JSON codec and token computation; separate Linux discovery/loading, durable edits/init, and CLI adapters after that.

Before filesystem work, revise the app ledger model/codec and W1 API from profile revisions to config tokens. Preserve the existing W1.1b ownership/loading and W1.1c atomic persistence split under the new transaction lifetime. Request integration, storage relocation, and queue/IPC changes follow S10a–S10d and C3 in that order; no frontend dependency is required for config work.

Test precedence, provenance, missing versus explicit values, unconfigured output, unlimited limits, exact selectors, bounds/types/Unicode, wrong-state binding, absent/corrupt/foreign files, no-op edits, concurrent writers, stale tokens after global/manual edits, failed/uncertain commits, CLI output failures, and immutable accepted requests. Use temporary roots and existing deterministic fixtures. Each implementation gets focused checks, `make check`, its documentation, and a STATUS update. Documentation-only contract work checks local links/anchors, examples, and whitespace; it establishes no runtime evidence.

Linux/amd64 is the first native target. Other platform configuration discovery/storage, daemon-state relocation, system-wide config, profile lifecycle CLI commands, generic environment overrides, richer budgets, and arbitrary extra settings remain deferred.
