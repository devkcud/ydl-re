# Project status

Updated: October 14, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) retains its stable Linux/amd64 release evidence. Milestones A through D and v0.2 E1 through E5 are complete locally.

Repository builds now expose the Linux daemon/queue CLI: explicit foreground start/stop/set, add/list/pause/resume/cancel/forget, safe text/JSON results, and independent standalone downloads. E6 forced-death/publication-boundary and final release qualification is next. Published `0.1.0` remains standalone; no v0.2 release or automatic daemon startup was added.

## Handoff

- `internal/cli/flags.go` shares the interspersed option scanner without echoing invalid values. `parseTransfer` in `cli.go` reuses download defaults/validation for add while rejecting overwrite and quiet. Existing download/inspect/version behavior remains intact.
- `daemon.go` implements start/stop/set through `Resolve`, `New`/`Run`, and `Call`. Start reports escaped state/socket paths and persistent runtime fallback, then admits work only after startup output succeeds. Failed readiness output closes ownership without dispatch, preserving already-committed recovery/settings. Clean stop exits 0; SIGINT/SIGTERM joins/checkpoints work and exits 130; persistence/ownership failures remain failures.
- `queue.go` generates random 128-bit lowercase-hex client IDs, normalizes requests, and resolves output names in the client's working directory before add. Calls are explicit, single-attempt IPC operations; no client creates daemon directories or starts a process. Output parents must exist when a job runs. The queue still never overwrites completed outputs.
- `queue_output.go` owns safe text and version-1 queue JSON results. All client controls accept `--json`; daemon start is text only. Each record contains `schema_version`, `command`, nullable `job_id`, `exit_code`, safe `result`, and nullable fixed `error`. This is separate from download progress/results and IPC framing. Add's text result is only its ID; list escapes nonprinting output paths and preserves unknown counters/publication.
- Lost/malformed/mismatched acknowledgments retain the submitted ID and `outcome_unknown`; calls do not retry automatically. Known remote refusals retain fixed failure codes. Failed/blocked client output cannot cancel daemon-owned jobs and reports the job ID on stderr when possible. Syntax/normalization errors remain stderr/exit 2; terminal JSON errors use a separate bounded reporting context. Inspect both output and process exit status.
- `cmd/yoinkdl/daemon_linux_test.go` runs actual entry-point processes with isolated XDG roots. It covers clean restart, preserved concurrency, reset startup rate, caller-relative outputs, conditional pause/resume, signals, broken/full acknowledgment pipes, cancel/forget preservation, and failed startup output cleanup. E6 still owns forced death and ambiguous publication boundaries.
- E4 remains in `internal/daemon`: pure path resolution, private Linux roots/locks, `/proc/self/fd`-anchored sockets, kernel peer-UID checks, strict bounded frames, sixteen accepted clients, fixed safe replies, and joined lifecycle. New prepares/binds without dispatch; Run executes once; Close signals independently of the control mailbox. Non-Linux daemon operations explicitly fail.
- E1/E2 durable queue rules and E3 body budgets are unchanged: intents before effects, FIFO admission, concurrency `1..8`, retained cleanup slots, explicit interrupted-job resume, output reservations, private full source URLs, terminal URL removal, metadata-only forget, and one retained shared budget across job/control changes.
- [CLI commands/results](docs/cli.md#daemon-and-queue), [daemon contract](docs/daemon-and-queue.md), and [testing evidence](docs/testing.md) own the detailed contracts. Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. Keep focused local commits; never push.

## Validation

- Parser extraction passed `go test ./internal/cli -run 'Test(Commands|DownloadOptions|InvalidArguments|ConnectionOptions|RetryOptions|RateOptions|InvalidOptions)' -count=1` and `make check` before its local commit.
- E5 passed `go test ./cmd/yoinkdl ./internal/cli -run 'Test(Daemon|Queue)' -count=1` and `make check`: formatting, vet, ordinary/race tests, and build. The changed CLI/process and packaging suites ran; unchanged daemon/queue/transfer/storage/range suites reused matching cached results. There were no E5 product test failures.
- `CGO_ENABLED=0 go test ./cmd/yoinkdl ./internal/cli -count=1` passed. Local HTTP/Unix sockets and process fixtures ran outside the sandbox. Native runtime evidence remains Linux/amd64; foreign binaries were not executed.
- The CI six-target recipe passed with CGO disabled: `go build -p 2 -trimpath` for the CLI and `go test -p 2 -c` for every test package on Linux/macOS/Windows amd64/arm64. Outputs are under `/tmp/yoinkdl-e5-compile.zrqbt3eo`.
- Tests verify exact HTTP output/checkpoints, durable controls, source redaction, no overwrite/implicit startup, generated IDs, unknown mutation outcomes, wrong-ID reply refusal, both text/JSON modes, output failure preserving work, real SIGINT/SIGTERM cleanup, and clean restart semantics.
- `python3 /tmp/yoinkdl-doccheck.py` passed 25 Markdown files, 344 local links/anchors, 36 shell blocks, eight JSON and two TOML examples, fences, and whitespace. `git diff --check` passed, including new files once staged. A draft trailing blank line was corrected. E6 forced-death/publication-boundary and final release gates were not run because that qualification is the next ordered step.

## Next steps

1. Implement deterministic E6 daemon crash fixtures: preserve queued order/settings and reconcile active intents after forced process death, with explicit resume and exact output checks. Cover a kill after verified publication but before queue completion; use a narrow test seam if needed and prove existing final files are preserved without inferred success or overwrite.
2. Qualify retained queue capacity/forget recovery and whole-process concurrency/shared-rate behavior, including restart and cancellation. Keep these tests distinct from existing in-process coordinator/budget evidence; do not reset shared credit during live controls.
3. Run final native ordinary/race/CGO-disabled process gates and six-target compilation, refresh security/platform/release documentation and this handoff, and split local commits. Prepare v0.2 release evidence locally; preserve existing tags/assets and never push.

## Open decisions and blockers

No E5 blocker remains. R03/R04, Linux paths, private URL persistence, and concurrency preferences are settled. E6 forced-death/publication-boundary and final release qualification remain unfinished. Native platforms beyond Linux/amd64 and real power-loss durability are unverified. Signing and package-manager distribution remain separate work.
