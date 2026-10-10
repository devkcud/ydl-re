# Resume and storage

[Documentation index](README.md)

Milestone B implements versioned manifests, durable prefix checkpoints, local checksum validation, exclusive ownership, validated HTTP resume, inspection, and recovery after interrupted publication. See [STATUS.md](../STATUS.md) for the current handoff.

## Files and ownership

For the destination `archive.zip`, storage uses:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

Data and state files use private `0600` permissions. Partial data stays beside the destination, on the same filesystem. Parent directories must exist. Destination symlinks, directories, special files, invalid UTF-8 paths, and reserved artifact names are rejected.

The empty lock file is persistent. Every writer holds a nonblocking OS advisory lock on its inode until all writes and cleanup finish. Closing the descriptor or process termination releases ownership; a leftover lock file alone does not mean a writer is active. Keeping the inode avoids an unlink/recreate race between competing writers. Do not manually delete it while a transfer may be active. [Linux flock semantics](https://man7.org/linux/man-pages/man2/flock.2.html).

The `.yoinkdl.part`, `.yoinkdl.json`, and `.yoinkdl.lock` suffixes and `.yoinkdl.tmp-` filename segment are reserved. Temporary manifests use unique names, are never recovery authority, and can remain after a crash. There is no automatic orphan-cleanup command.

Linux is tested locally. macOS operations are enabled but still need native qualification. Windows storage is unsupported. The filesystem must support advisory locks, hard links, and directory sync; network filesystems and power-loss guarantees are not qualified.

## Version 1 manifest

The schema represents one sequential prefix. This example records the three bytes `abc` from a ten-byte representation:

```json
{
  "schema_version": 1,
  "destination": "/home/user/Downloads/file",
  "source_id": "18c6f551ea3482332a3ecb8399217217eb214fd68dab3c888af753a504929c82",
  "effective_id": "18c6f551ea3482332a3ecb8399217217eb214fd68dab3c888af753a504929c82",
  "length": 10,
  "etag": "\"version-1\"",
  "encoding": "identity",
  "completed": 3,
  "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  "state": "partial"
}
```

| Field | Contract |
|---|---|
| `schema_version` | Exactly `1`; unknown versions are refused |
| `destination` | Exact absolute output path |
| `source_id` | SHA-256 of the normalized, fragment-free original URL |
| `effective_id` | SHA-256 of the fragment-free effective URL after redirects |
| `length` | Representation length, or `-1` while unknown |
| `etag` | A usable strong ETag or the empty string |
| `encoding` | `identity` or `encoded`; encoded bodies are saved unchanged |
| `completed` | Number of durable, hashed prefix bytes |
| `sha256` | SHA-256 of that prefix |
| `state` | `partial` or `ready` |

Every field must occur exactly once and be non-null. Unknown or missing fields, duplicate keys, invalid values, trailing JSON, and manifests larger than 64 KiB are rejected. The format does not contain original URLs, query tokens, request headers, or credentials.

URL fingerprints require the same URL spelling after parsing and fragment removal. Refreshed signed URLs and changed redirect targets are not silently treated as the same identity. A URL fingerprint is not a content checksum or an authentication mechanism.

## Interval convention

`completed = n` means the half-open prefix `[0, n)`. There is no inclusive endpoint in this schema. A partial checkpoint for a positive known length must end before that length; only a verified `ready` checkpoint can claim the entire file.

The original design's `completed_ranges` example was illustrative and used inclusive bounds. It is not schema version 1 and is not imported. Disjoint range persistence will require an explicit schema evolution during the parallel-transfer milestone.

## Checkpoint ordering

While receiving data, storage checkpoints after four additional MiB or one second of arriving data, whichever happens first. Normal shutdown forces a checkpoint where safe.

1. Write received bytes and update the incremental prefix digest.
2. Sync the data file before advancing saved progress.
3. Write and sync a uniquely named temporary manifest.
4. Publish the manifest atomically and sync the directory.
5. Remove the temporary name when possible.

The final bytes of a known-length response are not recorded as full coverage until the application verifies response completion. A read failure after those bytes therefore leaves an earlier checkpoint, forcing conservative re-fetching.

Unacknowledged data can extend beyond the checkpoint after a crash. Recovery verifies the saved prefix hash and bounds, and does not truncate that tail until the application confirms a compatible remote response. A rejected resume leaves data and metadata unchanged.

## Resume requirements

Saved bytes require a valid manifest, the same destination/source identity, an intact partial file, and a matching prefix checksum. An old Milestone A partial without a manifest is refused; its length is not proof of downloaded coverage.

Network resume additionally requires a known length, identity encoding, a usable strong ETag, and a valid compatible response from the server. Unknown-length, encoded, weak-tagged, or unversioned partial transfers are retained for inspection but cannot reuse bytes. Last-Modified alone is not a resume validator in this milestone.

Resource changes, unsupported ranges, changed effective URLs, corrupt state, and ambiguous artifacts are refused. There is no implicit destructive restart. Use another output path or deliberately abandon existing artifacts after confirming no writer owns them.

## Final publication and recovery

After successful transport and length checks, storage syncs the complete file and publishes a `ready` manifest with its full checksum. Only then does it publish the final output.

Default publication uses a hard link that cannot replace an existing destination. Explicit overwrite uses rename. Directory sync occurs before removing the partial name; the manifest is removed last. Errors retain whether verified output was already published.

| Recovery state | Behavior |
|---|---|
| `partial` manifest and valid prefix | Candidate for validated network resume |
| `ready` manifest and intact complete partial | Can finish publication without downloading again |
| `ready` with final/partial names for the same inode | Reconcile completed hard-link publication and cleanup |
| `ready` with no partial and a matching complete final file | Verify full checksum and finish cleanup after rename |
| Changed/missing bytes, unsupported metadata, or unrelated destination | Refuse; preserve evidence and existing files |

Local `ready` recovery does not infer success from a `416` response or file length. It requires the previously recorded complete checksum and matching request identity. It can complete an already received resource version even if the server is now unavailable.

A persistent lock remains after success; partial and manifest files are removed. Completed-download history and automatic stale-artifact cleanup are not implemented.
