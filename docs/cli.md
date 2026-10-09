# CLI reference

[Documentation index](README.md)

All commands and flags on this page are proposed. There is no parser or executable implementation yet.

## Commands

| Command | Purpose | Planned release |
|---|---|---|
| `yoinkdl download <url>` | Download one HTTP(S) resource to a local file | v0.1 |
| `yoinkdl inspect <output>` | Read local transfer metadata associated with an output path | v0.1 |
| `yoinkdl version` | Display the installed program version | v0.1 |

`yoinkdl download <url>` is the canonical scriptable interface. The shorter `yoinkdl <url>` alias is only a possible later convenience.

## Download

```text
yoinkdl download <url> [options]
```

The URL must use HTTP or HTTPS. The output is a local file. Batch inputs, standard-output streaming, custom range selection, authentication flags, and multiple URLs in one invocation are not specified for v0.1.

### Options

| Option | Intended behavior | Proposed default |
|---|---|---|
| `--output <path>`, `-o <path>` | Select the final destination | Safe URL path filename, or `download`, in the current directory |
| `--connections <count>`, `-c <count>` | Request up to this many range workers | `4` |
| `--limit <rate>` | Cap the aggregate transfer rate | Unlimited |
| `--resume` | Request reuse of matching existing transfer state | Automatic when compatible partial state exists |
| `--overwrite` | Allow replacement of an existing completed output | Off |
| `--timeout <policy>` | Configure request/idle timeout behavior | Exact syntax and values undecided |
| `--retries <count>` | Bound retries for transient range failures | `3` per range |
| `--json` | Produce machine-readable results/events | Off |
| `--quiet` | Show errors and final output only | Off |

The precise retry-attempt counting rule, timeout grammar, worker limits, and interaction of `--json` with `--quiet` are tracked in [open decisions](decisions.md#command-and-output-decisions).

### Output paths

An explicit output path takes precedence over filename derivation. A server's `Content-Disposition` must not override a user-selected path. Derived names must be safe for the local filesystem.

The transfer writes into a separate partial file beside the destination. A file at the final path must not be treated as a successful new download until verification and publication finish.

### Resume and overwrite

| Local situation | Intended behavior |
|---|---|
| No destination or partial state | Start a new transfer |
| Compatible manifest and partial file | Resume only after validating resource identity and recorded progress |
| Incompatible or corrupt partial state | Refuse unsafe reuse; preserve existing data until a deliberate restart policy applies |
| Completed destination exists | Fail unless `--overwrite` was supplied |
| Another process owns the transfer | Refuse a second writer |

`--resume` does not waive identity checks. `--overwrite` does not make mismatched intervals compatible. Their combined behavior, and explicit `--resume` with no partial state, still need a parser and policy decision.

### Examples

Save with a chosen name:

```sh
yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip
```

Request one worker:

```sh
yoinkdl download 'https://example.org/archive.zip' -c 1
```

One worker limits concurrency; it does not necessarily disable range requests used for safe resumption.

Request quiet output:

```sh
yoinkdl download 'https://example.org/archive.zip' --quiet
```

Request machine-readable output:

```sh
yoinkdl download 'https://example.org/archive.zip' --json
```

The JSON schema, event framing, stdout/stderr routing, and compatibility policy are not defined yet. Do not build automation around invented example fields.

## Inspect

```text
yoinkdl inspect <output>
```

The argument identifies the intended final output, such as `./archive.zip`. The command is intended to display associated local metadata. It should not require a daemon.

Candidate information includes destination, transfer mode, known size, persisted progress, validators, and stored state. These are design expectations, not finalized output fields. A local inspection alone cannot establish that the server still serves the same resource.

The behavior for missing or corrupt manifests, and flags accepted by `inspect`, remain open. Rich history is planned for v0.3.

## Version

```sh
yoinkdl version
```

Display the installed version. Exact formatting and whether commit/build metadata is included are undecided.

## Terminal behavior and exit status

The human display should show progress without counting retried bytes twice. Unknown totals suppress percentage and ETA. On success, print the final output path. On failure, explain the cause and whether recovery data remains.

Exit code `0` means success. Nonzero means failure. Dedicated error codes and the exact interrupt exit code are not reserved yet.

Progress is advisory. The final result must reflect verification and filesystem publication, not just receipt of an HTTP success status.

## Commands reserved for later

Daemon, queue, and TUI commands are listed in the [roadmap](roadmap.md#later-front-ends). They are not part of the v0.1 interface. There is no planned v0.1 cleanup, profile, hook, checksum, or arbitrary `--range` command/option.
