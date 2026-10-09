# Troubleshooting

[Documentation index](README.md)

The CLI is not implemented in this checkout yet. Apart from that current limitation, the situations below describe expected behavior and recovery guidance for the planned product, not observed runtime diagnostics.

## The command or build is unavailable

There is no executable or `go.mod` to run today. Installing Go alone will not create YoinkDL. Follow the [development guide](development.md) if implementing the project; the [getting-started examples](getting-started.md) describe the future interface.

## Download and network issues

| Situation | Expected explanation | Useful next step |
|---|---|---|
| Only one worker is active | Unknown length, unsuitable encoding, unproven ranges, or a small object can require fewer workers | Treat one stream as a valid fallback; inspect available metadata |
| Percentage or ETA is absent | The server did not provide a reliable total | Use transferred bytes and speed; wait for verified completion |
| Speed is lower than requested | `--limit` is a ceiling, not a target; the server, network, disk, or worker overhead may be slower | Check the configured cap and compare with fewer workers |
| `404` | The resource is unavailable at that URL | Verify the direct file URL; blind retries should not continue |
| `429` or transient `503` | The server is limiting or temporarily unable to serve requests | Allow bounded backoff; reduce requested concurrency if appropriate |
| Short body, wrong length, invalid range | The response cannot establish a complete interval | Keep valid recovery data and retry only through the engine's safe policy |
| TLS or proxy failure | Certificate verification or proxy connectivity failed | Correct the certificate/proxy environment; no insecure-TLS bypass is specified |

An HTTP success status does not necessarily mean the server sent the file the user intended. v0.1 has no content-specific authentication flow or checksum option. Verify the source URL and access requirements when a server returns an unexpected resource.

## A transfer will not resume

Use the same intended output path and keep both sidecars. The planned command is:

```sh
yoinkdl download 'https://example.org/archive.zip' --output ./downloads/archive.zip --resume
```

Potential reasons for refusing reuse include:

- The server serves a different representation or lacks a usable validator.
- Range support or encoding changed.
- The manifest is corrupt, unsupported, or incompatible with the destination.
- The partial file is absent or inconsistent with saved coverage.
- Another process owns the transfer.

The precise failure messages and restart interaction are not decided. `--resume` must not bypass validation, and `--overwrite` must not make incompatible partial state safe. A new output path can preserve the old artifacts while starting a fresh transfer.

If a signed URL expires, obtain a valid source URL through the normal provider flow. Whether a refreshed URL may reuse old state depends on a future identity policy; do not edit the manifest to force a match.

## The destination already exists

The default is to preserve it and fail. Choose another path or deliberately use `--overwrite` for a completed output you intend to replace.

Destination conflicts can also arise while a transfer is running. The final publication step must enforce the same policy; an earlier successful path check is not enough.

## Disk, permission, and ownership failures

Check that the output parent exists, is writable, and has space for the transfer and metadata. Free space or correct permissions without deleting valid partial artifacts automatically. Resume only after the underlying problem is corrected.

If another writer is active, stop or wait for that process through its normal lifecycle. Do not delete its lock while it may still be writing. No stale-lock recovery command or algorithm has been selected yet.

## Progress went backward after a crash

The display may include received bytes newer than the last durable checkpoint. Re-fetching those bytes after a crash is expected under a conservative recovery policy. It is safer than trusting data whose persistence cannot be established.

## Progress reached the end but finalization failed

Receiving all expected bytes is followed by verification, syncing, closing, and publication. Disk errors, destination conflicts, or platform filesystem behavior can still prevent completion.

Keep the partial file and manifest for diagnosis. Do not rename the partial file manually or assume a progress display proves integrity. The command must report whether the verified data remains available for recovery.

## A final file and manifest both remain

Publication may have succeeded before metadata cleanup. The recovery policy must distinguish this case from an interrupted download. Do not truncate the final file or force resumption based on leftover metadata alone.

The exact cleanup/error result is an [open decision](decisions.md#storage-and-recovery-decisions). `inspect` is planned for local state, but no automatic cleanup command is part of v0.1.

## Information for a useful report

Once an issue-reporting channel exists, include the program version, OS/filesystem, sanitized command, error text, whether another writer was active, and which final/partial/manifest files remain. State whether the failure followed a normal interrupt, network drop, or forced termination.

Remove credentials, signed URL tokens, sensitive local paths, and private downloaded content before sharing diagnostics. There is no selected security contact or persistent log format yet.

For the recovery model behind this guidance, read [resume and storage](resume-and-storage.md).
