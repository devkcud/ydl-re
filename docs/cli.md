# CLI reference

[Documentation index](README.md)

Milestones A and B implement the commands below. Examples use the built executable from the repository root. See [getting started](getting-started.md) to build it.

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
| `--overwrite` | Allow replacing an existing regular completed file | Off |
| `--resume` | Require existing saved transfer metadata | Compatible existing state resumes automatically without the flag |
| `--quiet` | Suppress progress; retain errors and final output | Off |
| `--timeout <duration>` | Timeout for connection, TLS, response headers, or an idle body read | `30s` |
| `--help`, `-h` | Display download help | Off |

Timeout values use Go duration syntax, such as `30s`, `500ms`, or `2m`, and must be positive. The timeout is not a limit on total download duration. Slow transfers can continue while body reads keep making progress.

### Output paths and artifacts

The default name comes from the original URL, ignoring query parameters and server-supplied filename suggestions. Unsafe separators/control characters are replaced, reserved device names are adjusted, and unusable or oversized names fall back to `download`.

Explicit output paths are resolved to absolute paths. Parent directories are not created automatically. Destination directories, symlinks, special files, and invalid UTF-8 paths are rejected. The suffixes `.yoinkdl.part`, `.yoinkdl.json`, `.yoinkdl.lock`, and the `.yoinkdl.tmp-` segment are reserved for transfer artifacts.

| Existing local state | Behavior |
|---|---|
| No destination or artifacts | Start a new transfer |
| Completed regular destination | Refuse unless `--overwrite` is supplied |
| Compatible partial file and manifest | Validate saved bytes and remote identity before resuming |
| Version-2 partial range state | Preserve and refuse before network access; parallel resume is not implemented |
| Incompatible state or a partial without a manifest | Refuse; leave it untouched, including with `--overwrite` |
| Another writer owns the transfer | Nonblocking OS lock fails; no second writer proceeds |
| Verified `ready` checkpoint | Finish publication/cleanup locally after full-checksum validation |
| Destination appears during transfer | Default publication fails without replacing that file |

Failed transfers retain nonempty partial files and their metadata. Empty newly failed transfers are cleaned up; zero-byte verified ready state remains recoverable. A persistent lock file stays after success and is not proof that a process is active.

### Resume

Repeat the same URL and output path to resume compatible state automatically:

```sh
./bin/yoinkdl download 'https://example.org/archive.zip' -o ./archive.zip
```

To require an existing checkpoint, add `--resume`. Without a manifest this fails before making a request. `--resume=false` uses the normal automatic policy; it is not a destructive restart or a way to discard saved data.

Partial reuse requires a known length, identity encoding, a matching strong ETag, exact URL identities, valid byte bounds, and intact local checkpoint bytes. Refreshed signed URLs, changed redirects, missing validators, and ignored ranges are refused. `--overwrite` only authorizes completed-destination replacement and does not bypass these checks.

Ready-state recovery may publish a fully received earlier resource version without contacting the server. See [resume and storage](resume-and-storage.md) for the precise recovery contract.

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

Progress and errors go to stderr. For a known size, progress shows transferred/total bytes, percentage, average rate, estimated remaining seconds, and one worker. Unknown-size transfers show bytes and rate without percentage or ETA.

Updates are throttled to about 200 ms while data arrives, with a final snapshot. Percentage includes validated saved bytes; speed counts only newly received bytes. Terminal output uses a refreshed line; redirected progress uses separate lines. There is no heartbeat while a body read is stalled; the inactivity timeout still applies.

Success prints the absolute output path to stdout. Failure does not print a success path. It identifies preserved partial data and, if publication already happened before a sync/cleanup error, identifies that verified output separately.

A percentage is advisory. Completion still requires transport and length checks followed by file publication.

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

For version 2, saved bytes are the sum of validated disjoint chunks, not a contiguous prefix or resume offset. Inspection reports that partial range resume is not implemented; verified ready publication remains recoverable.

Inspection does not contact the server, hash the partial file, acquire writer ownership, or change artifacts. Eligibility is conditional; download validates local bytes and the remote representation before reuse. Missing/corrupt state exits `1`; wrong argument count exits `2`.

Successful transfers remove their manifests, so inspection is not a completed-download history command. Its human-readable layout is not a JSON/event API.

## Planned additions

These interfaces remain proposed and are rejected by the current CLI:

| Addition | Intended milestone/release |
|---|---|
| `--connections`, `--retries` | C: bounded ranged transfers and retries |
| `--limit`, `--json` | D: v0.1 hardening |
| Configuration files | Optional format still to implement and specify |
| Queue/daemon commands | v0.2 |
| TUI, profiles, history, optional hooks | v0.3 |

The planned ranged-worker preference is four, retries are three per range with exact counting still to settle, and the rate cap defaults to unlimited. JSON event framing remains open; manifest versions 1 and 2 are documented in [storage](resume-and-storage.md). See [open decisions](decisions.md) before implementing later additions.
