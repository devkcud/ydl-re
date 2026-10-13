# CLI reference

[Documentation index](README.md)

Milestones A, B, and C plus the Milestone D rate cap and JSON output implement the commands below. Examples use the built executable from the repository root. See [getting started](getting-started.md) to build it.

## Commands

| Command | Behavior |
|---|---|
| `yoinkdl download <url> [options]` | Download one HTTP(S) resource |
| `yoinkdl inspect <output>` | Show local checkpoint metadata without contacting the server |
| `yoinkdl version` | Print `yoinkdl` and the build version; default `dev` |
| `yoinkdl help` | Show command help |
| `yoinkdl help download` | Show download help |
| `yoinkdl help inspect` | Show inspection help |

No arguments, `-h`, and `--help` also show command help. The shorthand `yoinkdl <url>` is not supported.

## Download

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' --output ./archive.zip
```

Options can precede or follow the URL. Supply exactly one absolute HTTP or HTTPS URL. Quote URLs with query parameters. Credentials embedded in URLs are rejected.

### Options

| Option | Behavior | Default |
|---|---|---|
| `--output <path>`, `-o <path>` | Final output path; parent must exist | Safe filename from the original URL path, or `download`, in the current directory |
| `--connections <n>`, `-c <n>` | Maximum active range workers, from 1 to 32; one streams new files directly or resumes saved ranges with one worker | `4`, reduced to the number of missing chunks |
| `--retries <n>` | Additional attempts for transient request/probe failures and incomplete ranges, from 0 to 10 | `3`; zero disables retries |
| `--limit <rate>` | Aggregate body rate across the transfer, such as `5MB/s` or `5MiB/s` | Unlimited |
| `--overwrite` | Allow replacing an existing regular completed file | Off |
| `--resume` | Require existing saved transfer metadata | Compatible existing state resumes automatically without the flag |
| `--json` | Newline-delimited progress and one terminal result on stdout | Off |
| `--quiet` | Suppress progress; retain errors and final output | Off |
| `--timeout <duration>` | Timeout for connection, TLS, response headers, or an idle body read | `30s` |
| `--help`, `-h` | Display download help | Off |

Timeout values use Go duration syntax, such as `30s`, `500ms`, or `2m`, and must be positive. The timeout is not a limit on total download duration. Slow transfers can continue while body reads keep making progress. Deliberate rate/backoff waits are excluded from body inactivity timeouts.

Connection counts use unsigned decimal digits, with leading zeros permitted. Zero, signs, fractions, hexadecimal, overflow, and values above 32 are rejected before network or file work. Fresh transfers with more than one connection probe range support and fall back to one stream when appropriate. Existing sequential checkpoints always resume sequentially.

Retry counts also use decimal digits, with leading zeros allowed. Three retries means at most four attempts for one operation. A chunk shares its budget across request and body failures. Sequential request setup can retry, but an interrupted sequential body keeps its checkpoint for a later resume. Integrity changes and local storage errors are terminal. See [retry policy](concurrency.md#retries) for statuses, backoff, and `Retry-After` limits.

Rates use a positive ASCII integer immediately followed by `B/s`, `KB/s`, `MB/s`, `GB/s`, `KiB/s`, `MiB/s`, or `GiB/s`. Units are case-sensitive; fractions, whitespace, signs, zero, and overflow are rejected before file/network work. Leading zeros are allowed. Omit `--limit` to disable the cap. See [unit values and bounds](configuration.md#units).

The cap shares one budget across workers, probes, redirects read by the HTTP client, and retry bodies. Saved bytes do not consume it, and retry progress rollback never refunds traffic. Initial/idle credit is 100 ms rounded up to a whole byte and capped at 64 KiB. Headers, framing, TLS, and socket read-ahead are excluded. See [aggregate rate accounting](concurrency.md#aggregate-rate-cap).

### Output paths and artifacts

The default name comes from the original URL, ignoring query parameters and server-supplied filename suggestions. Unsafe separators and nonprinting Unicode characters, including invisible/bidirectional formatting, are replaced. Printable localized names remain supported, known device names are adjusted, and unusable names or names over 200 bytes fall back to `download`. An explicit output path remains exact and is escaped only for display.

Explicit output paths are resolved to absolute paths. Parent directories are not created automatically. Destination directories, symlinks, special files, and invalid UTF-8 paths are rejected. The suffixes `.yoinkdl.part`, `.yoinkdl.json`, `.yoinkdl.lock`, and the `.yoinkdl.tmp-` segment are reserved for transfer artifacts.

| Existing local state | Behavior |
|---|---|
| No destination or artifacts | Start a new transfer |
| Completed regular destination | Refuse unless `--overwrite` is supplied |
| Compatible partial file and manifest | Validate saved bytes and remote identity before resuming |
| Version-2 partial range state | Validate saved chunks and remote identity, then fetch whole missing chunks |
| Incompatible state or a partial without a manifest | Refuse; leave it untouched, including with `--overwrite` |
| Another writer owns the transfer | Nonblocking OS lock fails; no second writer proceeds |
| Verified `ready` checkpoint | Finish publication/cleanup locally after full-checksum validation |
| Destination appears during transfer | Default publication fails without replacing that file |

Failed transfers preserve useful partial state. Newly failed empty sequential transfers are cleaned up; initialized parallel state remains even with no completed chunks. Zero-byte verified ready state remains recoverable. A persistent lock file stays after success and is not proof that a process is active.

### Resume

Repeat the same URL and output path to resume a compatible sequential or parallel checkpoint automatically:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip
```

To require an existing checkpoint, add `--resume`. Without a manifest this fails before making a request. `--resume=false` uses the normal automatic policy; it is not a destructive restart or a way to discard saved data.

Partial reuse requires a known length, identity encoding, a matching strong ETag, exact URL identities, valid byte bounds, and intact local checkpoint bytes. Refreshed signed URLs, changed redirects, missing validators, and ignored ranges are refused. `--overwrite` only authorizes completed-destination replacement and does not bypass these checks.

Ready-state recovery may publish a fully received earlier resource version without contacting the server. See [resume and storage](resume-and-storage.md) for the precise recovery contract.

Parallel resume keeps completed chunks and the saved chunk size. It validates the saved server identity before starting missing work. You can change `--connections` between runs; setting it to one processes saved range tasks with one worker and does not convert or discard the checkpoint.

### Examples

Suppress progress:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --quiet
```

Allow a longer inactivity interval:

```sh
./bin/yoinkdl download --timeout 2m 'https://example.org/archive.zip' -o ./archive.zip
```

Replace a completed file deliberately:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --overwrite
```

## Terminal output

Progress and errors go to stderr. For a known size, progress shows transferred/total bytes, percentage, average rate, estimated remaining seconds, and the active worker count. Unknown-size transfers show bytes and rate without percentage or ETA. Workers reach zero before final verification/publication.

Updates are throttled to about 200 ms while data arrives, with a final snapshot showing zero active workers after transfer cleanup. Percentage includes validated saved bytes; speed counts only newly received bytes. A detected terminal with nonempty `TERM` other than `dumb` uses a refreshed line, clipped below its reported width to avoid wrapping. Width is checked again for each frame. Redirected output, nonterminal devices, and plain terminals use separate lines without ANSI controls. There is no heartbeat while a body read is stalled; the inactivity timeout still applies.

Before replacing a failed chunk, its received bytes are removed from progress so retries cannot exceed the total or inflate speed. A worker waiting in backoff still occupies a worker slot. Backoff waits are cancellable and separate from network inactivity timeouts.

Success prints the absolute output path to stdout. Failure does not print a success path. It identifies preserved partial data and, if publication already happened before a sync/cleanup error, identifies that verified output separately.

Text output quotes and escapes paths, error text, and version labels that contain nonprinting characters, including line breaks, escape sequences, and bidirectional formatting characters. Ordinary printable paths retain their existing format. This changes only display; the filesystem name stays exact. Use `--json` for unambiguous machine-readable paths.

Help, version, inspection, progress, and diagnostics use the same bounded writer as JSON. Linux/macOS pipes, sockets, and detected terminals use owned descriptors above stdin/stdout/stderr, with five-second write deadlines and cancellation during live output. Each final report gets its own write budget after transfer cleanup. Disk files and custom writers without deadline support retain their underlying I/O behavior.

Text downloads check stdout setup before file/network work and initialize stderr only when progress is enabled. `--quiet` can succeed with stderr unavailable. A failed or short progress write cancels the transfer; workers join and storage closes before a best-effort diagnostic. Failure to print the final path or final progress after publication exits nonzero and identifies the verified file when stderr is usable. Output failures never delete published data or redirect diagnostics into stdout.

Help/version/inspection write failures exit `1`. Invalid commands/options retain exit `2` even if their diagnostic cannot be written. Caller interruption during blocked output exits `130`. An unavailable diagnostic stream cannot provide preservation messages; the nonzero exit still applies. Plain terminal output remains separate from JSON's versioned data contract.

Invalid option values are never echoed. Diagnostics identify the known option and refer to help for its accepted values; unknown option names are omitted too. This applies before either text or JSON transfers begin.

A percentage is advisory. Completion still requires transport and length checks followed by file publication.

## JSON output

`download --json` implements the version-1 format below. Other commands retain their current text output; `--json=false` selects normal text download output.

For a machine-readable terminal result without progress:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip --json --quiet
```

JSON mode emits newline-delimited UTF-8 objects on stdout, with no array, terminal control codes, or extra success-path line. Progress records are advisory; one terminal `result` follows transfer cleanup. `--quiet` suppresses progress only. Command/option parsing errors use stderr and exit `2`; help remains text. After parsing succeeds, transfer validation errors (including invalid URLs), failures, and interruption belong in the result, with no duplicate human error on stderr. Output-stream failures use stderr because stdout may be unusable.

Each record includes `schema_version: 1`, an `event` discriminator, and integer `elapsed_ms` since the event stream started. Version 1 keeps existing field types and meanings stable; additive fields and nonterminal events may appear. Consumers should ignore unknown fields/events, reject unsupported schema versions, and require a `completed` result with `published: true` plus process exit `0` for success. Error messages are human-readable diagnostics, not stable error codes. JSON integers may reach signed 64-bit values; consumers must preserve integer precision.

| Field | Progress | Terminal result |
|---|---|---|
| `event` | `progress` | `result` |
| `bytes` | Useful reported bytes, including saved bytes; can decrease on retry | Final reported counter; failure counts are not verified coverage |
| `total` | Expected body length, or `null` if unknown | Last observed expected length, or `null` |
| `resumed` | Saved bytes adopted for this invocation | Final adopted saved-byte count |
| `workers` | Active worker slots, including backoff | Absent; workers have joined |
| `status` | Absent | `completed`, `failed`, or `interrupted` |
| `exit_code` | Absent | Transfer outcome: `0`, `1`, or `130` |
| `path` | Absent | Absolute output path acquired by this invocation, or `null` |
| `partial` | Absent | Known retained partial path after cleanup, or `null` |
| `published` | Absent | Whether this invocation published verified output, even if later cleanup failed |
| `error` | Absent | Diagnostic string on failure/interruption; `null` on success |

Unknown totals remain `null`, including after a clean unknown-length download. Zero means a known empty body. Counters are not checkpoints, resume offsets, or rate-budget accounting. An early failure can have no observed progress; it may report locally saved bytes before remote validation. Null paths mean this invocation has no confirmed path to report, not that pre-existing artifacts are absent. Likewise, `published: false` does not claim an existing destination is absent.

Example records for a ten-byte transfer:

```json
{"schema_version":1,"event":"progress","elapsed_ms":0,"bytes":0,"total":10,"resumed":0,"workers":1}
```

```json
{"schema_version":1,"event":"result","elapsed_ms":20,"status":"completed","exit_code":0,"path":"/tmp/file","partial":null,"bytes":10,"total":10,"resumed":0,"published":true,"error":null}
```

Progress is emitted at most every 200 ms while snapshots arrive. Synchronous writes apply backpressure without an event queue or detached writer goroutine. Linux/macOS pipe/socket/terminal output uses an owned duplicate descriptor: writes have a five-second deadline and progress writes honor cancellation. Terminal reporting gets a separate bounded write so interruption can still produce a result. Disk writes and caller-supplied writers without deadline support retain their underlying I/O behavior.

A failed or short write stops further records and cancels transfer work. An interrupted write that delivered no bytes can still be followed by the terminal result; a partially written record makes the stream unusable. A broken stream can end with a truncated line or no result. Output failure exits `1`, or `130` when the caller interrupted, and stderr reports known publication/partial paths. Failure after publication preserves that verified file, even if the success record could not be delivered. Source URLs, redirect targets, headers, and validators are not event fields.

## Exit status

| Code | Meaning |
|---|---|
| `0` | Success or help |
| `1` | Download validation, network, filesystem, or final-output failure |
| `2` | Invalid command, argument count, or option |
| `130` | Transfer interrupted through the cancellation context, including SIGINT/SIGTERM |

## Inspect

```sh
./bin/yoinkdl inspect ./archive.zip
```

The path names the intended final output. The command reads the versioned manifest and prints destination, checkpoint state, saved bytes, total bytes or `unknown`, encoding, and resume eligibility. It can read a published snapshot while another process owns the transfer.

For version 2, saved bytes are the sum of validated disjoint chunks, not a contiguous prefix or resume offset. Resume eligibility is conditional on local checksums and matching remote identity; verified ready publication remains recoverable offline.

Inspection does not contact the server, hash the partial file, acquire writer ownership, or change artifacts. Eligibility is conditional; download validates local bytes and the remote representation before reuse. Missing/corrupt state exits `1`; wrong argument count exits `2`.

Successful transfers remove their manifests, so inspection is not a completed-download history command. Its human-readable layout is not a JSON/event API.

## Planned additions

These interfaces remain proposed and are rejected by the current CLI:

| Addition | Intended milestone/release |
|---|---|
| Configuration files | Optional format still to implement and specify |
| Queue/daemon commands | v0.2 |
| TUI, profiles, history, optional hooks | v0.3 |

The JSON event schema is separate from manifest versions 1 and 2, documented in [storage](resume-and-storage.md). See [open decisions](decisions.md) before implementing later additions.

### Planned v0.2 controls

The [daemon/queue contract](daemon-and-queue.md) is accepted but these commands are not implemented or accepted by the current CLI:

| Planned command | Behavior |
|---|---|
| `daemon start [--parallel N] [--limit R]` | Foreground owner of the queue and local socket; optional durable concurrency override and process-lifetime shared body cap |
| `daemon stop` | Acknowledge durable shutdown intent; stop admission and checkpoint/join active work |
| `daemon set --parallel N` | Persist the whole-transfer admission limit (`1..8`, initial default `1`) and apply it live |
| `add <url> [options]` | Persist an immutable normalized request and return its job ID; no overwrite option |
| `list [--json]` | Poll queue policy and safe job/progress views, omitting URLs |
| `pause <id>` | Pause waiting work or durably request active cancellation/checkpointing |
| `resume <id>` | Put a paused/interrupted/failed job at the FIFO tail; never bypass checkpoint/identity checks |
| `cancel <id>` | Stop future work, retaining files and clearing terminal source data |
| `forget <id>` | Remove a record without an active slot and free its reservation/capacity; never delete output or sidecars |

`--parallel 1` means one download at a time; `--parallel 4` allows four downloads. Each download can independently use `--connections 4` for up to four range workers. Lowering parallelism lets existing jobs finish and gates new admissions; explicitly pause extras for an immediate reduction.

Add accepts the current output, connections, retries, timeout, per-transfer limit, and require-resume options; output paths resolve in the invoking client's directory. No command implicitly starts a daemon. Successful pause/cancel/stop replies acknowledge durable intent, not instantaneous worker/process termination. If a control reply is lost, the operation may have committed; inspect the queue rather than blindly repeating a mutation. A broken list/acknowledgment output does not cancel durable jobs.
