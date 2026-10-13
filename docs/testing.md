# Testing and validation

[Documentation index](README.md)

Milestones A, B, and C plus Milestone D rate, JSON, text-output, security regression, and packaging work pass the local full gate on Go 1.27.2 and hosted Ubuntu 24.04.5 CI on Go 1.27.0, Linux/amd64. The matrix below also covers remaining platform/release work and must not be read as fully implemented.

## Current evidence

| Test file | Coverage |
|---|---|
| [Storage](../internal/storage/file_test.go) | Publication, overwrite, destination races, exclusive writers, retained partials, unsafe paths, sync failure |
| [Checkpoints](../internal/storage/checkpoint_test.go) | Durable prefixes, uncommitted tails, corrupt/invalid metadata, local-byte hashes, stable locks, interrupted link/rename publication |
| [Range planner](../internal/ranges/ranges_test.go) | Bounded fixed chunks, missing-work subtraction, exact coverage over completion subsets, invalid intervals, HTTP endpoints, maximum offsets |
| [Range checkpoints](../internal/storage/range_checkpoint_test.go) | Out-of-order completion, gap preservation, checksum/sync/publication failure, corrupt chunks, exact coverage, ready/link/rename recovery |
| [Range writers](../internal/storage/range_writer_test.go) | Exclusive claims, bounded offset writes, failed/canceled commits, stale writers, concurrent checkpoint serialization, cancellation before publication |
| [Range manifests](../internal/storage/range_manifest_test.go) | Distinct version-1/version-2 formats, strict nested JSON, chunk bounds, identity requirements, 256-chunk metadata capacity |
| [Range-state compatibility](../internal/transfer/range_state_test.go) | Ignored-range refusal, unchanged artifacts, offline ready recovery |
| [Transfer](../internal/transfer/download_test.go) | Fixture hashes, known/unknown/encoded/empty bodies, bad statuses, truncation, cancellation, timeouts, redirects, publication failure |
| [Request normalization](../internal/transfer/request_test.go) | Shared pure defaults, URL/filename identity, owned retry values, explicit-path preservation, and matching standalone validation order/redaction |
| [HTTP policy](../internal/transfer/http_test.go) | Safe derived names, cross-origin credential stripping, downgrade and credential rejection |
| [Representation identity](../internal/transfer/representation_test.go) | Immutable metadata snapshots, conditional requests, source/version/redirect consistency, exact range bounds, unknown totals, maximum offsets |
| [Capability probe](../internal/transfer/probe_test.go) | Validated one-byte GETs, ignored ranges, safe full-stream fallback, ineligible validators/encoding/length, 416 re-probing, malformed bodies, subsequent version changes |
| [Probe lifecycle](../internal/transfer/probe_lifecycle_test.go) | Header/body/completion timeouts, cancellation, request cleanup, bounded reads, fallback body ownership |
| [HTTP workers](../internal/transfer/workers_test.go) | Exact parallel fixture hashes, configured/default concurrency, chunked bodies, serialized progress, failure preservation, peer cancellation, invalid settings, cancellation before publication |
| [Resume](../internal/transfer/resume_test.go) | Automatic/explicit/chunked/zero-prefix resume; mismatched identities, bounds, and validators; unchanged-artifact refusal; 416 re-probing; offline ready recovery |
| [Range resume](../internal/transfer/range_resume_test.go) | Disjoint/empty/full coverage, conditional validation, whole missing chunks, changed identity/bounds, unchanged-artifact refusal, cancellation, interrupted-attempt replacement, one-worker resume |
| [Retry policy](../internal/transfer/retry_test.go) | Attempt bounds, status/network classification, jitter bounds, Retry-After parsing/ceiling, cancellation, cloning, cleanup, and secret redaction |
| [Retry integration](../internal/transfer/retry_integration_test.go) | Whole-chunk replacement, shared header/body budgets, progress rollback, terminal integrity/storage errors, new timeout contexts, bounded workers during backoff, fallback budgets, sequential resume setup |
| [Rate budget](../internal/transfer/limit_test.go) | Strict rate grammar/bounds, shared 1/4/32-reader allowance, bounded idle credit, cancellation without refunds, minimum/maximum rates |
| [Shared budgets](../internal/transfer/budget_test.go) | Independent clients/workers, private/shared/both caps, minimum/maximum rates, smallest read burst, latest reservation, canceled/error bytes charged to both budgets, and bounded credit across new clients and idle time |
| [Shared-budget HTTP](../internal/transfer/budget_http_test.go) | Concurrent downloads through the reusable runner, combined probe/body allowance, exact publication, private/shared cap composition, and nil/zero-value handles |
| [Shared-budget recovery](../internal/transfer/budget_recovery_test.go) | Offline version-1/version-2 ready recovery neither consumes nor resets shared credit |
| [Body pacing](../internal/transfer/body_test.go) | Bounded reads, throttle/timeout separation, later attempts after idle timeout, cancellation/cleanup, redirect-body accounting |
| [Rate integration](../internal/transfer/limit_integration_test.go) | Private and shared caps over fresh/saved probe and failed-chunk traffic, saved-byte exclusion, progress rollback, fallback continuity, canceled-worker joining, and real HTTP sequential/ranged resume, redirects, retries, and unknown-length streams |
| [CLI](../internal/cli/cli_test.go) | Argument ordering, invalid options, help/version, progress, quiet output, overwrite, error codes |
| [Text rendering](../internal/cli/render_test.go) and [Linux terminals](../internal/cli/terminal_linux_test.go) | Escaped paths/errors/build labels, unchanged actual filenames, redirected frames, nonterminal devices, final worker counts, real pseudo-terminal resizing and TERM=dumb |
| [Text output](../internal/cli/text_test.go) | Failed/short help, version and inspection writes; read-only inspect; progress failure before/after publication; checkpoint ownership release; blocked reply/diagnostic cancellation; quiet mode with unavailable stderr; coalescing after backpressure |
| [JSON records](../internal/cli/events_test.go) | Exact integer counts, null/empty totals, framing/escaping, progress coalescing/rollback, quiet mode, result statuses, publication/cleanup distinction, short writes |
| [Output deadlines](../internal/cli/output_test.go) and [pipe ownership](../internal/cli/output_unix_test.go) | Controlled deadline/cancellation, deadline cleanup, duplicate ownership, original flag restoration, full-pipe cancellation |
| [JSON integration](../internal/cli/json_test.go) | Known/unknown/empty/parallel output, sequential/ranged resume with retries, publication refusal and offline recovery, output failures before/after publication, source redaction, help/argument/text compatibility |
| [Process](../cmd/yoinkdl/main_test.go) | Entry point, successful download, real SIGINT with retained partial bytes and no final output |
| [HTTPS process](../cmd/yoinkdl/https_linux_test.go) | Linux CLI sequential/ranged downloads with a trusted local TLS certificate; untrusted TLS refusal without HTTP requests, final/partial data, or a manifest, with per-process CA isolation |
| [Crash recovery](../cmd/yoinkdl/resume_test.go) | Kill a process after a durable checkpoint, inspect its state, resume in another process, and verify the complete fixture hash |
| [Parallel process](../cmd/yoinkdl/parallel_test.go) | Actual CLI process, connection flag, conditional range requests, and complete fixture checksum |
| [Parallel crash recovery](../cmd/yoinkdl/range_resume_test.go) | Kill with disjoint completed chunks and wrong unconfirmed bytes, restart with a different worker limit, fetch only whole missing chunks, verify final hash and cleanup |
| [Retry process](../cmd/yoinkdl/retry_test.go) | Actual CLI default of three additional attempts, explicit one-retry budgets, zero disabling retries, and exact output |
| [Rate process](../cmd/yoinkdl/limit_test.go) | Actual CLI aggregate `8MiB/s` cap across three chunks of a 9 MiB fixture, success timing before process teardown, and exact output checksum |
| [JSON process](../cmd/yoinkdl/json_test.go) and [pipe failures](../cmd/yoinkdl/json_pipe_test.go) | Real SIGINT result and checkpoint/resume, quiet result, broken stdout before/after publication, closed stderr, full-stdout cancellation with cleanup before draining |
| [Text process](../cmd/yoinkdl/text_pipe_test.go) | Broken help/stdout/stderr, diagnostic exit status, JSON fallback with both streams broken, quiet with unusable stderr, merged streams, and SIGTERM during full stderr with checkpoint ownership released before draining |
| [CLI security](../internal/cli/security_test.go) | Invalid scalar/boolean values carrying synthetic credentials never echo their contents or start transfer work |
| [HTTP security](../internal/transfer/security_test.go) and [transport process](../cmd/yoinkdl/security_test.go) | Malformed trailer redaction, rejected full-response range headers, unchanged zero-prefix artifacts, and no unsolicited server bytes in transport logs |
| [Storage security](../internal/storage/security_test.go) | Refused hard-linked/shared-permission mutable partials, late-created aliases, no writes to other names, and exact stored-byte verification before sequential publication |
| [Release packaging](../tools/release_test.go) | Real binary/source archives, checksum/source identity, extracted-binary downloads, byte reproducibility, dirty-tree/version/output refusal, and build/smoke-failure cleanup on native Linux/amd64 |
| [Queue model](../internal/queue/model_test.go) and [codec](../internal/queue/codec_test.go) | E1 pure normalization, immutable hash encoding, state/source combinations, private errors, empty/full snapshots, limits, duplicate/null/unknown fields, and lossless Unicode |
| [Queue store](../internal/queue/store_linux_test.go) | Private creation, restart round trips, settings, deep-copy isolation, terminal URL removal, immutable requests/order, unsafe/corrupt state refusal, stale-file preservation, and changed-artifact detection |
| [Queue commit failures](../internal/queue/store_faults_linux_test.go) | Partial/short writes, file sync, rename, directory sync, acknowledgment ordering, uncertain-commit refusal, retryable pre-rename failures, and safe temporary cleanup |
| [Queue ownership](../internal/queue/store_lock_linux_test.go) | Concurrent opens, real competing processes, forced owner death, persistent lock inode, and unchanged saved running intent on load |
| [Queue lifecycle](../internal/queue/lifecycle_test.go) | Every pause/resume/cancel state, recovery, FIFO-tail resume and overflow, publication precedence, and fixed safe error messages |
| [Queue coordinator](../internal/queue/coordinator_linux_test.go) | FIFO dispatch, persisted recovery and startup settings, running intent before engine calls, safe owned views, draining slots, signal shutdown, and no automatic replay |
| [Coordinator failures](../internal/queue/coordinator_faults_linux_test.go) | Failed preparation/admission/results/stop, uncertain commits, joined cancellation without stale saves, bounded mailboxes, disconnected callers, and progress during blocked persistence |
| [Queue controls](../internal/queue/controls_linux_test.go) | Live limits through eight transfers, non-preemptive reductions, persisted settings, idempotent adds, output reservations, FIFO-tail resume, preserved files, draining slots, and late publication |
| [Queue capacity](../internal/queue/control_limits_linux_test.go) | Full retained queues, forget freeing capacity, admission-order exhaustion, and encoded-size refusal before IO without stopping the queue |
| [Control commits](../internal/queue/control_commits_linux_test.go) | Admission only after a setting commit, disconnected add persistence/deduplication, and caller request isolation |
| [Queue HTTP integration](../internal/queue/coordinator_http_linux_test.go) | Default engine runner, real pause/checkpoint/conditional resume, exact published bytes, terminal URL removal, and existing-output refusal |
| [Queue shared-budget integration](../internal/queue/budget_http_linux_test.go) | One retained budget across real pause/resume, failure/retry, cancellation, completion, subsequent jobs, and live concurrency increases/reductions; exact output and retained partial data |
| [Daemon ownership](../internal/daemon/ownership_linux_test.go) | XDG/default/fallback paths, length bounds, both lock combinations, private modes, stale/live/unsafe/replaced sockets, changed roots/locks, simulated foreign artifact owners, and actual kernel peer credentials |
| [Daemon protocol](../internal/daemon/protocol_test.go) | Every request/reply shape, nested duplicates/unknown/null fields, normalized transfer input, Unicode and size bounds, fixed redacted errors, and exact newline frame boundaries |

`make check` runs formatting checks, vet, tests, race tests, and the binary build. The built binary's version and download help were also exercised. Tests use local sockets and need an environment that permits localhost listeners.

The built executable also passed a workspace-filesystem smoke test: interrupt, inspect the saved checkpoint, resume from 128 KiB, verify the complete fixture hash, and confirm that URL tokens were absent from metadata.

The parallel crash test waits for two disjoint chunk checkpoints and unconfirmed bytes in both gaps, then forcibly kills the writer. A new process revalidates the saved identity, uses two workers instead of four, replaces only the missing chunks, and produces the exact fixture hash. The partial already has the full logical file length, so the test also rejects size-based completion shortcuts. Saved metadata contains no raw URL tokens.

JSON process tests verify actual stdout descriptor behavior. Broken pipes return exit `1`, preserve state, and report known artifacts when stderr is available; exit status and preservation still hold when stderr is closed. A full-pipe test interrupts while progress output is blocked, proves partial cleanup completes before the reader drains, then receives the interrupted result. A SIGINT/resume test validates the checkpoint checksum and final fixture hash. Post-publication storage-cleanup failure is covered at the record-mapping boundary; broader storage fault injection remains separate qualification.

The security review first reproduced option-value and HTTP-parser disclosure, raw unsolicited HTTP/1 transport logging, contradictory full-response acceptance, writes through hard-linked/shared-permission partials, publication of altered sequential disk bytes, and nonprinting URL-derived names. Focused regressions and the final full gate pass after the fixes. Existing interrupted link/rename recovery still works for both manifest versions. See [review evidence and boundaries](security.md#review-evidence).

Real disk exhaustion, broader sync/cleanup fault injection, power-loss durability, and native macOS/Windows qualification remain incomplete. Fresh/resumed parallel transfers, bounded retries, aggregate body-rate caps, concurrent offset writers, and checkpoint serialization have executable tests. Retry unit/integration fixtures substitute controlled waits where timing itself is not under test; CLI process checks exercise real backoff. Process termination is not a real power-loss test; publication-boundary recovery also uses controlled ready/link/rename states.

The October 13 E1/E2 queue tests pass on Linux/amd64 with ordinary, race, and CGO-disabled execution. `make check` passes, and the CI compilation recipe builds the CLI and all test programs for Linux/macOS/Windows on amd64/arm64 with CGO disabled. Queue storage is Linux-only; foreign binaries were not run. Controlled runners prove the coordinator's lifecycle, bounds, durability, and failure ordering; the real HTTP test exercises the existing downloader through pause/resume and no-overwrite refusal. The CGO-disabled run required an environment allowing localhost sockets after the sandbox refused its listener. These checks do not establish daemon IPC/process lifecycle or real power-loss behavior.

E3 passed focused controlled-clock and local HTTP checks, `make check`, and `CGO_ENABLED=0 go test ./internal/transfer ./internal/queue -count=1` on Linux/amd64. Four independent clients with four readers each satisfy private/shared allowances at minimum, rounded, and maximum rates. Canceled/error bytes retain both charges; combined waits use the later reservation. Real downloads cover probes, redirects, fallback, retries, and saved network bytes; ready recovery consumes no credit. A real queue retains the same cap across job lifecycle and concurrency changes. No daemon process or new foreign-target runtime qualification is implied.

## Test layers

| Layer | Purpose | Suggested tools |
|---|---|---|
| Unit | Range arithmetic, coverage, parsing, retry classification, manifest validation | Go `testing`, deterministic inputs |
| HTTP integration | Correct and hostile server behavior with exact output assertions | `httptest.Server`, local fixtures |
| Process recovery | Interrupt/relaunch, concurrent ownership, crash checkpoints | Subprocesses and temporary directories |
| Platform qualification | Filesystem publication, locks, signals, permissions | Native Linux/macOS/Windows runs |

The design favors local servers over public internet URLs. Fixture bytes and expected hashes must be known before the transfer begins.

## Required scenarios

| ID | Scenario | Required evidence |
|---|---|---|
| T01 | Basic `200` stream | Exact final bytes, successful completion, no prematurely published partial |
| T02 | Correct `206` responses | Exact output; intervals cover the full object without overlap or holes |
| T03 | Server ignores `Range` and returns `200` | Safe sequential fallback; no full body written into a range offset |
| T04 | Invalid `Content-Range`, wrong lengths, short body, midstream EOF | No success; invalid interval remains incomplete |
| T05 | ETag changes between runs or responses | No mixing of versions; safe refusal/restart policy |
| T06 | Interrupt and relaunch | Only valid checkpointed bytes reused; exact final output |
| T07 | Connection drops during a range | Safe interval replacement/retry without duplicate accounting |
| T08 | Multiple workers under a rate cap | Measured aggregate throughput respects the defined allowance |
| T09 | Unknown-length response | Success only after clean transport completion; no fictional total/ETA |
| T10 | Destination collision, permissions, disk full | Clear failure; completed files and valid partial data preserved |
| T11 | Corrupt manifest or simultaneous writers | Unsafe state rejected; only one writer owns a transfer |
| T12 | `429`/`503` and permanent `404` | Bounded backoff and appropriate `Retry-After`; no blind permanent-error loop |
| T13 | Concurrent workers and shutdown | Race detector clean on exercised paths; separate resource/goroutine leak checks |
| T14 | Identity encoding and unexpected compression | No combining data with incompatible byte offsets |
| T15 | Missing/weak validators or ineligible dates | No unsafe cross-response or saved-byte reuse |
| T16 | Redirects and sensitive headers | Bounded redirects, correct effective identity, no indiscriminate credential forwarding |
| T17 | Empty files, one-byte files, boundary intervals, `416` | Correct coverage and re-probing; no off-by-one completion shortcut |
| T18 | Preallocation and overlapping saved ranges | File size or byte sum alone cannot establish completeness |
| T19 | Crash during checkpoint/finalization | Recover only trustworthy data; reconcile final output and leftover metadata |
| T20 | Destination appears during transfer; symlinks and unsafe names | Publication and path policy enforced at the actual write boundary |
| T21 | CLI defaults, invalid options, quiet/JSON output, exit behavior | Matches the adopted command contract; errors identify preserved state |

T01–T13 cover the original design's testing scenarios, with filesystem cases split for clarity. T14–T21 expand the design's HTTP, safety, and interface requirements into explicit review targets.

## Range and coverage assertions

Test uneven splits, small objects, zero length, maximum supported offsets, reversed intervals, out-of-bounds endpoints, duplicates, overlaps, and gaps. Use the chosen manifest convention consistently and test conversion to HTTP inclusive endpoints.

Assert actual output bytes, not only file length. A sparse or preallocated file can have the expected length while lacking downloaded data. Equal summed range lengths can also conceal overlap and holes.

## Crash and durability injection

Exercise failures at these boundaries:

1. Data written but not yet eligible for a checkpoint.
2. Data made durable before manifest publication.
3. Temporary manifest written but not published.
4. New manifest published before normal shutdown.
5. Verified partial file closed before final publication.
6. Final file published before metadata cleanup.

The chosen storage strategy determines exact injection points. Tests must demonstrate that a checkpoint never trusts missing bytes and that a completed output is not truncated during recovery.

Inject write, sync, close, rename/publication, and cleanup failures through narrow storage boundaries. Do not fill a developer's real disk to exercise disk exhaustion.

## Cancellation and retry assertions

Cancellation must interrupt active requests, limiter waits, and retry timers. Wait for worker completion and check that open response bodies, files, and goroutines are released.

Use controlled time and randomness where practical for retry tests. Assert attempt bounds and scheduler ownership as well as eventual success. Rate tests use standard-library `testing/synctest` with in-memory HTTP transports to verify shared timing, including failed attempts and body timeout/cancellation. Clock rounding tolerance is one microsecond. Local HTTP/CLI tests allow five milliseconds of sampling tolerance against the initial burst plus elapsed-time allowance; they do not promise a minimum speed. The CLI test measures the success line before process teardown so race-runtime exit delays cannot hide a multiplied worker cap.

The race detector only finds races on executed paths. It does not replace crash, protocol, or leak testing. [Go race detector documentation](https://go.dev/doc/articles/race_detector).

## Commands

The Makefile exposes `make test`, `make vet`, `make race`, and the full `make check` gate. The underlying commands are:

```sh
go test ./...
go vet ./...
go test -race ./...
```

These checks pass locally on Linux. Use the toolchain selected by the project; race testing and native filesystem tests need suitable platform environments. Other operating systems have not been qualified by these results.

The [platform report](platforms.md) records the October 12 native Linux toolchain, ext4/tmpfs evidence, CGO-disabled run, exact source revision, and reproduction commands. The six CLI targets build, and all test packages compile for foreign targets, with CGO disabled. Foreign binaries were not executed; native runtime evidence remains separate from compilation results.

The [CI/release workflows](releases.md) passed local actionlint v1.7.12 checks and Bash syntax checks. Native CI commands and the six-target compilation loop were replayed locally. The final packaging gate passed, including race checks; separate shell fixtures reject malformed tags and missing notes and select only the intended changelog section. Optional ShellCheck/Pyflakes integrations were disabled because those tools were not installed. [Hosted CI](platforms.md#hosted-ci-evidence) subsequently passed the full gate, uncached CGO-disabled tests, and six-target compilation at `1ce24810b7b6a05607c30ab37ff725758e29ebb8`. The later [hosted candidate report](releases.md#hosted-candidate-verification) records successful tag-triggered draft creation and downloaded-asset checks.

## Planned v0.2 validation

These are required checks for the [accepted daemon design](daemon-and-queue.md). E1 storage, E2 coordinator/controls, and E3 shared-budget tests are recorded above, including safe internal views. IPC/CLI privacy and full daemon process checks remain pending. The [delivery order](roadmap.md#v02-implementation-order) determines when each group becomes executable.

| Group | Required evidence |
|---|---|
| Queue codec/store | Empty/full round trips; exact fields/types/versions; duplicate/null/oversized input; ID/order/output conflicts; immutable request hash; unsafe owners/modes/symlinks/hard links; corrupt files preserved |
| Durable mutation | Fail writes, file sync, rename, and directory sync separately; acknowledgment only after durability; uncertain post-rename failures stop work; no stale-memory overwrite; temporary files never adopted |
| Source privacy | Synthetic query tokens persist only in private source state, clear on terminal transitions/forget, and never reach text, JSON views, logs, malformed-request errors, or IPC failures |
| Custom concurrency | Default/persisted `1` runs serially; values through `8` bound jobs independently of per-job workers; increases admit FIFO; reductions keep active work but block excess admissions; settings survive restart; capacity and order overflow are refused |
| Lifecycle | Persist running/pause/cancel before effects; keep slots through cleanup; test cancel during verification/publication, published-with-error results, all restart transitions, and absence of automatic whole-job retry |
| Ambiguous publication | Kill after output publication but before queue completion; recover as unknown/interrupted, preserve final data, and never infer completion or overwrite through retry |
| Local IPC | Matching/mismatching peer identities and state roots; unsafe/stale/live sockets; both lock combinations; private fallback path; frame/version/shape limits; bounded clients and read/write deadlines |
| Lost replies | Drop a connection after durable add/settings/control commits; verify accepted work survives and IDs allow inspection; repeated add IDs deduplicate only while retained |
| Shared bandwidth | Controlled-clock and HTTP tests across multiple transfers; private/global/both limits; probes/redirects/retries; low/high rates; cancellation after reads; no refund or renewed global burst on resume or concurrency changes |
| Real processes | Explicit daemon lifecycle, signal cleanup, forced death, saved queue order and settings, real transfer resume, standalone writer conflicts, and isolated test roots |

Use local deterministic servers and temporary roots. Run focused checks for each slice, then `make check` for implementation changes. Native Linux and CGO-disabled process coverage is required; foreign compilation alone never proves daemon support. Existing v0.1 transfer, output, storage, TLS, and packaging suites remain regression gates.

## Release evidence

The [first candidate report](releases.md#first-candidate-verification) records `0.1.0-rc.1` at `5fae374cd31429f5d8653c6c497dcf0d21865c7e`: the full local packaging gate, archive checksums and exact source identity, isolated installation, and sequential/parallel fixture downloads passed. The hosted candidate passed the same downloaded-archive and installed-binary checks with Go 1.27.0; its source archive matches the local archive exactly. After publication, [unauthenticated public access and installation](releases.md#publication-and-public-downloads) passed with all three assets byte-identical to the verified draft.

The [stable review and archives](releases.md#stable-archive-verification) add real Linux HTTPS subprocess tests with isolated trust, then verify `0.1.0` at `31330c6b07d33af148341bdfb346c0e139082fdb`. Ordinary, race, and CGO-disabled HTTPS checks passed locally and in hosted Go 1.27.0 gates. Local, hosted, and publicly downloaded stable binaries passed HTTP/HTTPS one-/four-connection fixture downloads and untrusted-certificate refusal. [Stable publication](releases.md#stable-publication) also verified anonymous access, Latest routing, unchanged assets, and preservation of the prerelease.

For every release, record the revision, toolchain, native platform/filesystem, commands run, results, and any skipped cases. The evidence must cover direct download, safe resume, representation consistency, wrong-length detection, atomic publication, exclusive ownership, integration tests, and race checks.

A test report should distinguish a build from a runtime test and a normal interruption from a crash-recovery exercise. Cross-platform builds alone are insufficient evidence for all filesystem guarantees.

See [v0.1 acceptance](roadmap.md#v010-acceptance) and the [open decisions](decisions.md) that must be resolved to turn this matrix into executable assertions.
