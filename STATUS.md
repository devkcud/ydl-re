# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) is the published stable Linux/amd64 release. Milestones A through D remain complete within that scope. No open post-release issues were returned by the current GitHub check.

v0.2 design is active. The owner selected private persistence of full queued URLs, including query tokens, and requested configurable concurrent downloads. Queue/IPC/persistence contracts are recorded; daemon-wide bandwidth and the ordered implementation plan remain next. No daemon, queue store, or new command is implemented.

## Handoff

- [Daemon and persistent queue](docs/daemon-and-queue.md) owns accepted R04 contracts: Linux foreground process, owner-only pathname IPC, peer credentials, distinct state/runtime locks, strict atomic snapshots, conservative crash recovery, and bounded safe status replies. Standalone commands stay independent.
- Queue `parallel` persists, defaults to `1`, and accepts `1..8`; it is separate from `--connections` within one transfer. Startup can override it, and `daemon set --parallel N` will change it live. Lowering it limits new admissions while active jobs finish; no silent cancellation is implied.
- Queue files deliberately retain full URL/query tokens under private permissions; existing transfer manifests remain fingerprint-only. Safe response records never include stored URLs. Completion/cancellation clears the source; forget removes idle records without deleting files. The implementation must not describe these files as encrypted.
- Running intent is durable before calling `transfer.Download`; pause/cancel intent is durable before cancellation. Slots stay occupied until the engine returns. Restart normalizes active states conservatively and never infers completion from an existing final file. Initial queued transfers do not support overwrite.
- [Architecture](docs/architecture.md), [configuration](docs/configuration.md), [security](docs/security.md), and [decisions](docs/decisions.md) now distinguish accepted v0.2 contracts from v0.1 behavior. Existing Go APIs for Unix peer credentials and controlled socket unlinking are available without new dependencies.
- Preserve release tags `v0.1.0` at `31330c6` and `v0.1.0-rc.1` at `5fae374`, and all published assets. [Release evidence](docs/releases.md#stable-publication) owns the completed publication checks. This new feature work follows the default focused-local-commit/no-push workflow.

## Validation

- `gh issue list --repo devkcud/ydl-re --state open --limit 100 --json number,title,body,labels,url,updatedAt` returned no issues. No defect reproduction took priority over v0.2 design.
- Read current transfer request/result, limiter/body, storage-lock, and CLI boundaries to check reuse and recovery assumptions. Official XDG and Linux Unix-socket references support path and peer-credential choices; links are in the contract. No code or dependency changed, so no runtime gate was repeated.
- `python3 /tmp/yoinkdl-doccheck.py` passed: 25 Markdown files, 299 local links/anchors, 32 shell blocks, six JSON examples, two TOML examples, fences, and whitespace, including the new untracked contract. `git diff --check` passed. The original design remains unchanged; its illustrative shell placeholders and intentional hard-break spaces retain the existing audit treatment.

## Next steps

1. Define R03's shared daemon body-budget contract, including composition with per-transfer limits, cancellation, and creation of credit. Record the ordered implementation and failure-path validation plan before adding code.
2. Implement the resulting first dependency step: strict queue schema and private atomic persistence. No user-visible daemon commands should be advertised as working before the control path and transfer lifecycle are implemented.

## Open decisions and blockers

R04 and Linux daemon path selection are settled at design level, including the owner's URL-persistence and concurrency preferences. R03 and the implementation plan remain. Broader native qualification, durability, signing, configuration files, package-manager distribution, and v0.3 TUI/hooks remain separate work.
