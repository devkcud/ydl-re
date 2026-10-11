# Testing and validation

[Documentation index](README.md)

Milestones A and B have executable tests and a passing local `make check` on Go 1.27.2, Linux/amd64. The full matrix below also covers later milestones and must not be read as fully implemented.

## Current evidence

| Test file | Coverage |
|---|---|
| [Storage](../internal/storage/file_test.go) | Publication, overwrite, destination races, exclusive writers, retained partials, unsafe paths, sync failure |
| [Checkpoints](../internal/storage/checkpoint_test.go) | Durable prefixes, uncommitted tails, corrupt/invalid metadata, local-byte hashes, stable locks, interrupted link/rename publication |
| [Range planner](../internal/ranges/ranges_test.go) | Bounded fixed chunks, missing-work subtraction, exact coverage over completion subsets, invalid intervals, HTTP endpoints, maximum offsets |
| [Range checkpoints](../internal/storage/range_checkpoint_test.go) | Out-of-order completion, gap preservation, checksum/sync/publication failure, corrupt chunks, exact coverage, ready/link/rename recovery |
| [Range manifests](../internal/storage/range_manifest_test.go) | Distinct version-1/version-2 formats, strict nested JSON, chunk bounds, identity requirements, 256-chunk metadata capacity |
| [Range-state compatibility](../internal/transfer/range_state_test.go) | Partial range refusal before network access, unchanged artifacts, offline ready recovery |
| [Transfer](../internal/transfer/download_test.go) | Fixture hashes, known/unknown/encoded/empty bodies, bad statuses, truncation, cancellation, timeouts, redirects, publication failure |
| [HTTP policy](../internal/transfer/http_test.go) | Safe derived names, cross-origin credential stripping, downgrade and credential rejection |
| [Resume](../internal/transfer/resume_test.go) | Automatic/explicit/chunked/zero-prefix resume; mismatched identities, bounds, and validators; unchanged-artifact refusal; 416 re-probing; offline ready recovery |
| [CLI](../internal/cli/cli_test.go) | Argument ordering, invalid options, help/version, progress, quiet output, overwrite, error codes |
| [Process](../cmd/yoinkdl/main_test.go) | Entry point, successful download, real SIGINT with retained partial bytes and no final output |
| [Crash recovery](../cmd/yoinkdl/resume_test.go) | Kill a process after a durable checkpoint, inspect its state, resume in another process, and verify the complete fixture hash |

`make check` runs formatting checks, vet, tests, race tests, and the binary build. The built binary's version and download help were also exercised. Tests use local sockets and need an environment that permits localhost listeners.

The built executable also passed a workspace-filesystem smoke test: interrupt, inspect the saved checkpoint, resume from 128 KiB, verify the complete fixture hash, and confirm that URL tokens were absent from metadata.

Parallel HTTP transfers, worker ownership, retries, caps, real disk exhaustion, broader sync/cleanup fault injection, power-loss durability, and native macOS/Windows qualification remain incomplete. Range storage fixtures write deterministic offsets directly; they do not qualify a concurrent offset writer or HTTP scheduler. Publication-boundary recovery is exercised with controlled ready/link/rename states; that is not a real power-loss test.

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

Use controlled time and randomness where practical for retry tests. Assert attempt bounds and scheduler ownership as well as eventual success. A limiter test needs the chosen burst/timing tolerance before it can assert an exact cap.

The race detector only finds races on executed paths. It does not replace crash, protocol, or leak testing. [Go race detector documentation](https://go.dev/doc/articles/race_detector).

## Commands

The Makefile exposes `make test`, `make vet`, `make race`, and the full `make check` gate. The underlying commands are:

```sh
go test ./...
go vet ./...
go test -race ./...
```

These checks pass locally on Linux. Use the toolchain selected by the project; race testing and native filesystem tests need suitable platform environments. Other operating systems have not been qualified by these results.

## Release evidence

Before v0.1, record the revision, toolchain, native platform/filesystem, commands run, results, and any skipped cases. The evidence must cover direct download, safe resume, representation consistency, wrong-length detection, atomic publication, exclusive ownership, integration tests, and race checks.

A test report should distinguish a build from a runtime test and a normal interruption from a crash-recovery exercise. Cross-platform builds alone are insufficient evidence for all filesystem guarantees.

See [v0.1 acceptance](roadmap.md#v010-acceptance) and the [open decisions](decisions.md) that must be resolved to turn this matrix into executable assertions.
