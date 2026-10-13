# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) is the published stable Linux/amd64 release. Milestones A through D remain complete within that scope. No open post-release issues were returned by this session's GitHub check.

The v0.2 design is accepted; E1 implementation is in progress. Shared pure request normalization is implemented for the existing downloader and future queue. Queue schema/storage remain next. The owner's private full-URL persistence and configurable concurrent-download choices remain fixed. No daemon, new command, or dependency has been added.

## Handoff

- `internal/transfer/request.go` now exposes `Normalize` and shares its implementation with `Download`. It preserves validation order/messages, filename rules, explicit output paths, defaults, and flags; retry values are copied so normalized requests do not alias caller storage. It performs no filesystem/network work. Focused before/after checks, normalization regressions, and `make check` passed.
- [Daemon and persistent queue](docs/daemon-and-queue.md) is the canonical contract. Use a foreground Linux daemon, private pathname Unix IPC with peer-UID checks, distinct state/runtime locks, strict bounded atomic snapshots, and safe polling replies. Standalone commands never depend on or start the daemon.
- Persistent queue `parallel` defaults to `1` and accepts `1..8`, independently of per-transfer `--connections`. Startup may override the saved setting; `daemon set --parallel N` changes it live. Increasing it admits FIFO work; decreasing it lets active jobs finish and withholds new admissions. Pause/cancel cleanup still occupies a slot.
- Queue files intentionally retain full source URLs and query tokens under private permissions. Transfer manifests stay fingerprint-only. Safe public views exclude stored requests; completion/cancellation clears the URL, and forget removes idle metadata without deleting data files. No encryption or secure-erasure claim is made.
- Persist running intent before engine execution and pause/cancel intent before cancellation. Reconcile process death conservatively; an existing final file never proves a queue job completed. Interrupted attempts need explicit resume, and initial queue transfers cannot overwrite final destinations.
- [Shared bandwidth](docs/concurrency.md#daemon-wide-rate-cap) uses one daemon-lifetime budget, composed with each job's own cap. Charge both applicable budgets before waiting; cancellations, progress rollback, job turnover, or concurrency changes cannot restore shared credit. `daemon start --limit` is fixed until restart and not a stored queue setting. Standalone processes remain independent.
- [Implementation order](docs/roadmap.md#v02-implementation-order) is E1 queue model/store and shared input validation, E2 coordinator and controls, E3 budget composition, E4 local IPC/lifecycle, E5 CLI adapters, E6 process/platform qualification. [Planned validation](docs/testing.md#planned-v02-validation) lists the required failure paths. `internal/queue` and `internal/daemon` are planned responsibilities, not existing packages.
- Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and all published assets. [Release evidence](docs/releases.md#stable-publication) owns the completed checks. v0.2 feature work uses focused local commits; do not push without new authorization.

## Validation

- `gh issue list --repo devkcud/ydl-re --state open --limit 100 --json number,title,body,labels,url,updatedAt` returned no issues. No defect reproduction took priority over design.
- Read current transfer validation/request/result, limiter/body, file-lock, and CLI code to verify reuse boundaries and identify publication/queue-commit ambiguity. Official XDG and Linux Unix-socket references support the path/credential choices; links are in the canonical contract. Standard-library peer-credential and controlled-unlink APIs are available in the installed Go source.
- `python3 /tmp/yoinkdl-doccheck.py` passed: 25 Markdown files, 316 local links/anchors, 32 shell blocks, six JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. The original design remains unchanged; its illustrative shell placeholders and intentional hard-break spaces retain the existing audit treatment.
- The focused baseline and post-extraction commands passed: `go test ./internal/transfer ./internal/cli -run 'Test(Invalid|Filename|RedirectPolicy|Download$|Resume$|Parse)' -count=1`, then the same pattern including `Normalize`. `make check` passed formatting, vet, ordinary/race tests, and build. Transfer/CLI/process/tooling suites reran; unchanged storage/range packages reused matching results.

## Next steps

1. Implement the rest of E1 in `internal/queue`: strict version-1 snapshot/job validation, private state ownership, and atomic persistence using `transfer.Normalize`. Cover malformed/duplicate/null/oversized input, source privacy, path/ID/order conflicts, unsafe-file refusal, competing writers, and pre-/post-rename failures. Do not add a listener or advertise queue commands in this slice.
2. Run focused E1 tests and `make check`, update this handoff with actual implementation/evidence, and commit the coherent slice locally. Then implement E2's coordinator, durable concurrency changes, and conservative restart transitions.
3. Follow E3 through E6 in order. Broader native support, TUI/hooks, general configuration files, dynamic shared-rate changes, and cross-process bandwidth remain separate work.

## Open decisions and blockers

R03/R04 and Linux daemon path selection are settled, including both owner preferences. E1's queue model/store remain unfinished. The coordinator, shared daemon budget, IPC, and CLI controls are not implemented. Broader native qualification, filesystem durability, signing, and package-manager distribution are outside this slice.
