# Project status

Updated: October 13, 2026 (America/Sao_Paulo).

## Current state

[YoinkDL 0.1.0](https://github.com/devkcud/ydl-re/releases/tag/v0.1.0) is the published stable Linux/amd64 release. Milestones A through D remain complete within that scope.

E1 of v0.2 is complete locally: shared pure request normalization, strict queue model/codec, and private Linux atomic storage with exclusive ownership. Queue state persists `parallel` in `1..8`, default `1`, independently of per-transfer connections. E2 scheduling and controls are next. No daemon command, listener, new dependency, or automatic startup exists yet.

## Handoff

- `internal/transfer/request.go` exposes `Normalize`, shared with `Download`. Validation order/messages, filename rules, explicit output paths, and defaults remain compatible. It owns copied retry values and performs no filesystem/network work.
- `internal/queue/model.go` and `codec.go` implement snapshot version 1, bounded settings/jobs/strings/encoding, immutable request hashes, source retention/removal, and state/result consistency. `NewJob` requires an absolute clean output. Strict decoding rejects duplicate/unknown/missing/null fields, malformed/oversized/trailing JSON, and lossy Unicode escapes without echoing payloads.
- `store.go` and `files_linux.go` implement `Open`, `Snapshot`, `Save`, and `Close`. Open takes an explicit absolute clean state directory with an existing trusted parent; it creates the final directory as `0700`, anchors operations to `os.Root`, and holds a persistent nonblocking `queue.lock`. Files are created as `0600`; unsafe owners, modes, types, symlinks, and multiple hard links are refused without repair. Other platforms return `ErrUnsupported`.
- Save deep-copies and validates the candidate, preserves retained request identity and monotonic admission order, verifies the lock/canonical inode and content, writes/syncs an exclusive temporary file, renames, then syncs the directory before updating its cache. Ordinary pre-rename failures preserve committed state. `ErrChanged` and `ErrUncertain` disable snapshots and writes until close/reopen; ownership remains held until Close. No stale-memory rollback is allowed.
- Loading never adopts temporary files or resets corrupt state. A missing canonical file beside queue temporaries requires explicit recovery. Cleanup only removes an unchanged owned temporary inode; lock files and foreign/stale artifacts remain. Loading saved running intent does not reconcile it or call the engine: E2 must durably perform restart transitions before dispatch.
- The owner selected full source URLs, including query tokens, in private local queue files. Transfer manifests remain fingerprint-only. Completed/canceled records remove URLs; safe public views must omit stored requests. The store API returns private records for internal use, not CLI/IPC serialization.
- [Daemon and queue](docs/daemon-and-queue.md) owns the accepted concurrency, lifecycle, source, and IPC contracts. E2 must serialize whole-job admission, retain slots through pause/cancel cleanup, and preserve real publication outcomes. Lowering `parallel` withholds new admissions and lets active work finish; increasing it admits FIFO work. [Shared budgeting](docs/concurrency.md#daemon-wide-rate-cap) remains E3.
- Preserve `v0.1.0` at `31330c6`, `v0.1.0-rc.1` at `5fae374`, and published assets. [Release evidence](docs/releases.md#stable-publication) owns completed release checks. Feature work uses focused local commits only; never push.

## Validation

- Shared normalization passed the focused before/after transfer/CLI checks: `go test ./internal/transfer ./internal/cli -run 'Test(Invalid|Filename|RedirectPolicy|Download$|Resume$|Parse)' -count=1`, then the same pattern including `Normalize`. Its full `make check` gate passed.
- `go test ./internal/queue -count=1` passed after model/codec and again after storage implementation. Store fixtures initially used non-private testing directories and were correctly refused; fixtures now explicitly create private directories, and the full focused suite passes. Coverage includes strict decoding, capacity/encoded bounds, snapshot isolation, immutable identities, permissions/types/ownership, corrupt/stale state, competing processes, forced lock-owner death, and failure injection around commit boundaries.
- The latest `make check` passed formatting, vet, ordinary/race tests, and build. Queue and packaging suites reran; unchanged transfer, CLI, process, range, and transfer-storage suites reused matching cached results. `CGO_ENABLED=0 go test ./internal/queue -count=1` also passed.
- The CI six-target compilation recipe passed with CGO disabled: `GOOS`/`GOARCH` combinations for Linux/macOS/Windows on amd64/arm64, using `go build -p 2 -trimpath` for the CLI and `go test -p 2 -c` for all test packages. Outputs are under `/tmp/yoinkdl-e1-compile.fqnXbi`. Foreign binaries were not executed; no foreign native queue support is claimed.
- `python3 /tmp/yoinkdl-doccheck.py` passed: 25 Markdown files, 320 local links/anchors, 32 shell blocks, six JSON examples, two TOML examples, fences, and whitespace. `git diff --check` passed. The original design remains unchanged; its illustrative placeholders and intentional Markdown hard breaks retain the existing audit treatment.

## Next steps

1. Implement E2's coordinator around the existing store and a narrow callable transfer runner. Start with deterministic fake-runner tests for persisted startup reconciliation, FIFO admission, durable running intent, and slot ownership. Then implement add/pause/resume/cancel/forget and durable live concurrency changes, including increases/decreases and publication/cancellation races.
2. Validate E2 with focused tests and `make check`, update this handoff, and split coherent local commits. Do not add listener/CLI commands before the queue lifecycle is proven.
3. Follow E3 shared budgets, E4 Linux paths/private IPC/lifecycle, E5 CLI adapters, then E6 process/platform qualification in the [ordered plan](docs/roadmap.md#v02-implementation-order).

## Open decisions and blockers

R03/R04, Linux path selection, and both owner preferences are settled. No E1 implementation blocker remains. E2 through E6 are unimplemented. Native platforms beyond Linux/amd64, real power-loss durability, signing, and package-manager distribution remain separate qualification work.
