# CLI reference

[Documentation index](README.md)

Milestone A implements the commands below. Examples use the built executable from the repository root. See [getting started](getting-started.md) to build it.

## Commands

| Command | Behavior |
|---|---|
| `yoinkdl download <url> [options]` | Download one HTTP(S) resource |
| `yoinkdl version` | Print `yoinkdl` and the build version; default `dev` |
| `yoinkdl help` | Show command help |
| `yoinkdl help download` | Show download help |

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
| `--quiet` | Suppress progress; retain errors and final output | Off |
| `--timeout <duration>` | Timeout for connection, TLS, response headers, or an idle body read | `30s` |
| `--help`, `-h` | Display download help | Off |

Timeout values use Go duration syntax, such as `30s`, `500ms`, or `2m`, and must be positive. The timeout is not a limit on total download duration. Slow transfers can continue while body reads keep making progress.

### Output paths and artifacts

The default name comes from the original URL, ignoring query parameters and server-supplied filename suggestions. Unsafe separators/control characters are replaced, reserved device names are adjusted, and unusable or oversized names fall back to `download`.

Explicit output paths are resolved to absolute paths. Parent directories are not created automatically. Destination directories, symlinks, and special files are rejected. The suffixes `.yoinkdl.part` and `.yoinkdl.json` are reserved for transfer artifacts.

| Existing local state | Behavior |
|---|---|
| No destination or artifacts | Start a new transfer |
| Completed regular destination | Refuse unless `--overwrite` is supplied |
| Partial file or manifest | Refuse; leave it untouched, including with `--overwrite` |
| Another writer owns the partial path | Exclusive creation fails; no second writer proceeds |
| Destination appears during transfer | Default publication fails without replacing that file |

Failed transfers retain nonempty partial files. Empty partials are removed. Resume is not implemented, so stored bytes are not reused automatically.

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

Updates are throttled to about 200 ms while data arrives, with a final snapshot. Terminal output uses a refreshed line; redirected progress uses separate lines. There is no heartbeat while a body read is stalled; the inactivity timeout still applies.

Success prints the absolute output path to stdout. Failure does not print a success path. It identifies preserved partial data and, if publication already happened before a sync/cleanup error, identifies that verified output separately.

A percentage is advisory. Completion still requires transport and length checks followed by file publication.

## Exit status

| Code | Meaning |
|---|---|
| `0` | Success or help |
| `1` | Download validation, network, filesystem, or final-output failure |
| `2` | Invalid command, argument count, or option |
| `130` | Transfer interrupted through the cancellation context, including SIGINT/SIGTERM |

## Planned additions

These interfaces remain proposed and are rejected by the current CLI:

| Addition | Intended milestone/release |
|---|---|
| `--resume`, `inspect <output>` | B: persistence and safe resume |
| `--connections`, `--retries` | C: bounded ranged transfers and retries |
| `--limit`, `--json` | D: v0.1 hardening |
| Configuration files | Optional format still to implement and specify |
| Queue/daemon commands | v0.2 |
| TUI, profiles, history, optional hooks | v0.3 |

The planned ranged-worker preference is four, retries are three per range with exact counting still to settle, and the rate cap defaults to unlimited. JSON framing and the manifest schema are not stable APIs. See [open decisions](decisions.md) before implementing these additions.
