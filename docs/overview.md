# Project overview

[Documentation index](README.md)

YoinkDL aims to turn a direct HTTP(S) URL into a correct local file. Users should be able to interrupt a transfer, resume it when the server allows that safely, and understand why a transfer failed.

**Product promise:** Yoink a file. Keep it intact. Pick up where you left off.

Stable `0.1.0` is published for Linux/amd64. Milestones A through D deliver sequential and bounded parallel downloads, verified checkpoints, safe resume, bounded retries, local inspection, progress, rate caps, JSON output, cancellation, and publication. See [project status](README.md#project-status), [installation](releases.md#install-a-published-binary), and the [current CLI](cli.md).

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

The standalone CLI runs in the foreground. Interrupting that process is its pause mechanism. The implemented [daemon and queue](daemon-and-queue.md) provide managed jobs with explicit pause/resume controls.

## Product scope

| State | Capabilities |
|---|---|
| Implemented standalone engine | `download`, `inspect`, `version`; safe HTTP(S) streaming/ranges, resume, retries, progress, rate caps, JSON output, and verified publication |
| Implemented daemon/queue | Private local IPC, durable admission and controls, bounded concurrency, shared bandwidth, and conservative restart behavior |
| Planned local web app | Vue 3 + TypeScript; downloads/queue, profiles, library/history, rules, schedules, Vue Flow Visual Grabber, activity/settings, and portable definitions in [ordered feature phases](roadmap.md#local-web-delivery-order) |
| Explicitly later integration | Browser link handoff, supported download interception, and self-hosted instance/profile selection such as Home server → Pato |
| Unscheduled directions | Other protocols, distributed transfers, peer discovery, and custom script/hooks |

The app is local-first and hosted-second. An instance selects the machine/output filesystem; a profile is a workspace within that instance. The extension and hosted targeting are not prerequisites for the initial web app. The TUI remains canceled, and versioning/publication is separate from the build roadmap.

The original draft's detailed CLI and hardening milestone place a per-transfer `--limit` in v0.1, which is implemented. The accepted [v0.2 design](daemon-and-queue.md) adds a separate shared daemon cap and a persistent whole-download concurrency setting: `1` for sequential work, higher values for concurrent downloads. The published Linux `0.2.0-rc.1` prerelease implements these settings and passed [local/hosted qualification and public installation](releases.md#v02-hosted-qualification-and-publication). [Budget composition](concurrency.md#daemon-wide-rate-cap) defines their boundary with the existing cap.

## Outside v0.1

- HLS/DASH media streaming, browser automation, scraping, captcha solving, protected-content bypassing, and DRM circumvention.
- FTP, SFTP, torrents, peer discovery, LAN P2P, and transfers spread across machines.
- Plugins, scripting engines, arbitrary workflows, automatic unpacking, and automatic execution of downloaded content.
- A daemon, database server, web dashboard, or persistent queue as a prerequisite for downloading a file.

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
