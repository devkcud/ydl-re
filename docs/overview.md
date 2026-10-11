# Project overview

[Documentation index](README.md)

YoinkDL aims to turn a direct HTTP(S) URL into a correct local file. Users should be able to interrupt a transfer, resume it when the server allows that safely, and understand why a transfer failed.

**Product promise:** Yoink a file. Keep it intact. Pick up where you left off.

Milestones A, B, and C implement sequential and bounded parallel downloads, verified checkpoints, safe resume, bounded retries, local inspection, progress, cancellation, and publication. See [project status](README.md#project-status) and the [current CLI](cli.md).

## Design principles

| Principle | Practical consequence |
|---|---|
| Correctness before throughput | Check response boundaries and final coverage before reporting success |
| Interruption is normal | Persist conservative checkpoints and preserve usable partial data |
| Graceful fallback | Use one stream when parallel byte ranges cannot be trusted |
| One engine, several front ends | Keep terminal rendering outside the transfer application service |
| Small operational footprint | Require no daemon or database for the first release |
| Simple configuration | Provide useful defaults and keep tuning optional |

## Intended v0.1 workflow

1. Supply a direct HTTP or HTTPS URL and optionally an output path.
2. Probe the server and choose a safe transfer mode.
3. Write into a partial file while reporting progress.
4. On interruption, retain safe progress for a later compatible resume.
5. Verify the completed transfer and publish the final file.

The CLI runs in the foreground. Interrupting that process is the initial pause mechanism. Persistent queue commands and background transfers arrive in a later release.

## Release scope

| Release | Intended capabilities |
|---|---|
| v0.1 | `download`, `inspect`, `version`; HTTP(S); safe streaming and parallel ranges; local resume; retries; progress; aggregate transfer cap; JSON output; completion checks |
| v0.2 | Local daemon, persistent queue, add/list/pause/resume/cancel operations, scheduling concurrent transfers, bandwidth management |
| v0.3 | TUI, named profiles, richer inspection and history, optional completion hooks |
| Later | Optional web UI, other protocols, remote nodes, LAN transfers, scheduling features, sandboxed workflows |

The draft's detailed CLI and hardening milestone place a per-transfer `--limit` in v0.1. Its release table also mentions bandwidth limits in v0.2. The boundary for daemon-wide limits remains an [open decision](decisions.md#release-and-compatibility-decisions).

## Outside v0.1

- HLS/DASH media streaming, browser automation, scraping, captcha solving, protected-content bypassing, and DRM circumvention.
- FTP, SFTP, torrents, peer discovery, LAN P2P, and transfers spread across machines.
- Plugins, scripting engines, arbitrary workflows, automatic unpacking, and automatic execution of downloaded content.
- A daemon, database server, web dashboard, persistent queue, or TUI as a prerequisite for downloading a file.

User-supplied checksum verification is also deferred. v0.1 still checks transfer length, range coverage, response validity, and successful completion. Tests compare fixture hashes even before a checksum option exists.

## Reliability boundaries

Resumption depends on compatible local state and trustworthy evidence that the remote representation is unchanged. A server advertising ranges is insufficient by itself.

Unknown-length responses can be downloaded sequentially, but the UI cannot show a meaningful total percentage or ETA. A clean end of such a response establishes transport completion; without an expected size or checksum, it cannot establish what size the publisher intended.

Linux/amd64 has local build, integration, and race-test evidence. macOS operations are enabled but need native qualification; Windows downloads are rejected until publication is implemented there. Broader filesystem crash-durability and platform qualification remain release work.

## Terms

| Term | Meaning here |
|---|---|
| Transfer | One attempt to obtain a resource version at a chosen destination, including retries and compatible recovery |
| Representation | The exact bytes selected by the server for a request |
| Validator | Server metadata used to test whether a representation changed, such as an eligible ETag |
| Range | A contiguous byte interval assigned to a request |
| Worker | A bounded task executor fetching one interval at a time |
| Partial file | Data that has not yet been verified and published as the final output |
| Manifest | Local metadata describing transfer identity and reusable progress |
| Checkpoint | A persisted record of progress that recovery can safely trust |
| Sidecar | A transfer metadata or data file beside the intended destination |
| Finalization | Publishing verified partial data at the requested final path |

Continue with [getting started](getting-started.md) or the [implementation roadmap](roadmap.md).
