# Troubleshooting

[Documentation index](README.md)

The CLI supports parallel downloads, safe sequential/range resume, bounded retries, checkpoint inspection, and interrupted-publication recovery.

## The command or build is unavailable

Run `make build` from the repository root, then use `./bin/yoinkdl`. The project needs Go 1.27 or newer and GNU Make; it is not automatically installed on your shell's PATH. See [getting started](getting-started.md).

If tests fail with a socket permission error, allow localhost listeners in the test environment. `make check` includes the race detector, which also needs a supported Go/CGO environment.

## Download and network issues

| Situation | Expected explanation | Useful next step |
|---|---|---|
| Only one worker is active | The file needs one chunk, the probe selected a stream, a sequential checkpoint is resuming, or the limit is one | More workers require enough chunks and validated range support |
| Percentage or ETA is absent | The server did not provide a reliable total | Use transferred bytes and speed; wait for verified completion |
| Speed is low | The server, network, proxy, or disk can limit transfer speed | Check those resources and the active worker count; more connections are not always faster |
| `404` | The resource is unavailable at that URL | Verify the direct file URL; blind retries should not continue |
| `429` or transient `503` | The server is limiting or temporarily unable to serve requests | The retry budget and bounded backoff apply; retry later if the budget is exhausted or `Retry-After` exceeds 30 seconds |
| Progress briefly decreases | A failed chunk is being replaced from its start | Completed checkpoints remain valid; failed-attempt bytes are removed from progress |
| Short body or wrong length | The response cannot establish a complete file | Retry the same URL/path to resume when identity permits, or choose a fresh path if state is incompatible |
| TLS or proxy failure | Certificate verification or proxy connectivity failed | Correct the certificate/proxy environment; no insecure-TLS bypass is specified |

An HTTP success status does not necessarily mean the server sent the file the user intended. v0.1 has no content-specific authentication flow or checksum option. Verify the source URL and access requirements when a server returns an unexpected resource.

## A transfer will not resume

Run `yoinkdl inspect <output>` to see saved metadata. Resume is automatic for compatible sequential or parallel state; `--resume` requires an existing manifest. Use the same original URL and output path and retain both the partial and manifest. Parallel resume validates a conditional response before it starts fetching whole missing chunks.

Reasons for refusal include:

- The server serves a different representation or lacks a usable validator.
- Range support or encoding changed.
- The manifest is corrupt, unsupported, or incompatible with the destination.
- The partial file is absent or inconsistent with saved coverage.
- Another process owns the transfer.

Unknown-length or encoded partials cannot resume, and strong ETags are required in both saved metadata and resumed responses. The current policy preserves incompatible artifacts and refuses an implicit restart. `--overwrite` permits replacing a completed regular file; it does not discard partial state.

If a signed URL expires, obtain a valid source URL through the normal provider flow. A refreshed URL or changed redirect target has a different fingerprint and is refused for old state. Use a fresh output path; do not edit the manifest to force a match.

## The destination already exists

The default is to preserve it and fail. Choose another path or deliberately use `--overwrite` for a completed output you intend to replace.

Destination conflicts can also arise while a transfer is running. The final publication step must enforce the same policy; an earlier successful path check is not enough.

## Disk, permission, and ownership failures

Check that the output parent exists, is writable, and has space for the transfer and metadata. Free space or correct permissions without deleting valid partial artifacts automatically. Resume only after the underlying problem is corrected.

If another writer is active, stop or wait for that process through its normal lifecycle. The OS lock is released on close or process death. A persistent `.yoinkdl.lock` file is expected even after success; its presence is not a stale-owner error. Do not unlink it while a writer may be active, since that can create competing lock inodes.

## A partial remains after a crash

A crash may leave bytes beyond the last sequential checkpoint or in incomplete parallel chunks. Sequential recovery verifies the prefix checksum and remote response before discarding only the unconfirmed tail. Parallel recovery verifies saved chunks, validates the remote version, and replaces each missing chunk in full. Refused validation preserves existing artifacts. Displayed progress can lead saved progress, and local checksum verification can take time.

A Milestone A partial without a manifest cannot be trusted or imported automatically. Choose another output path or deliberately abandon it when no process is active.

## Progress reached the end but finalization failed

Receiving all expected bytes is followed by verification, syncing, closing, and publication. Disk errors, destination conflicts, or platform filesystem behavior can still prevent completion.

Keep the partial file and manifest for diagnosis. Do not rename the partial file manually or assume a progress display proves integrity. The command must report whether the verified data remains available for recovery.

## A final file and manifest both remain

Publication may have succeeded before metadata cleanup. The recovery policy must distinguish this case from an interrupted download. Do not truncate the final file or force resumption based on leftover metadata alone.

The current CLI reports a failure and identifies the published output if a later directory-sync or cleanup operation fails. Repeat the same request to reconcile a `ready` checkpoint against complete local bytes and finish cleanup. A mismatched checksum or unrelated destination is refused. There is no automatic orphan-cleanup command.

## Information for a useful report

Once an issue-reporting channel exists, include the program version, OS/filesystem, sanitized command, error text, whether another writer was active, and which final/partial/manifest files remain. State whether the failure followed a normal interrupt, network drop, or forced termination.

Remove credentials, signed URL tokens, sensitive local paths, and private downloaded content before sharing diagnostics. There is no selected security contact or persistent log format yet.

For the recovery model behind this guidance, read [resume and storage](resume-and-storage.md).
