# Resume and storage

[Documentation index](README.md)

Milestone B implements durable prefix checkpoints, local checksum validation, exclusive ownership, validated HTTP resume, inspection, and recovery after interrupted publication. Milestone C adds parallel transfers and safe resume with exclusive chunk writers and version-2 checkpoints. See [STATUS.md](../STATUS.md) for the current handoff.

## Files and ownership

For the destination `archive.zip`, storage uses:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
archive.zip.yoinkdl.lock
```

New data and state files use private `0600` permissions. Partial data stays beside the destination, on the same filesystem. Parent directories must exist. Destination symlinks, directories, special files, invalid UTF-8 paths, and reserved artifact names are rejected.

Mutable partial files must have exactly one hard link and no group/other permission bits. Recovery, activation, writes, checkpoints, and preparation for publication check these constraints. An incompatible file is preserved without truncation, chmod, or unlinking its aliases, even with `--overwrite`. Verified `ready` recovery may retain the intentional final/partial hard links left by interrupted publication because it writes no content bytes. The caller must still trust the output directory and avoid external changes during a transfer; advisory locks do not exclude unrelated filesystem writers.

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
| `schema_version` | Exactly `1` for this prefix format |
| `destination` | Exact absolute output path |
| `source_id` | SHA-256 of the normalized, fragment-free original URL |
| `effective_id` | SHA-256 of the fragment-free effective URL after redirects |
| `length` | Representation length, or `-1` while unknown |
| `etag` | A usable strong ETag or the empty string |
| `encoding` | `identity` or `encoded`; encoded bodies are saved unchanged |
| `completed` | Number of durable, hashed prefix bytes |
| `sha256` | SHA-256 of that prefix |
| `state` | `partial` or `ready` |

Every field must occur exactly once, use the documented case, and be non-null. Unknown or missing fields, duplicate keys, invalid values, trailing JSON, and manifests larger than 64 KiB are rejected. The format does not contain original URLs, query tokens, request headers, or credentials.

URL fingerprints require the same URL spelling after parsing and fragment removal. Refreshed signed URLs and changed redirect targets are not silently treated as the same identity. A URL fingerprint is not a content checksum or an authentication mechanism.

## Interval convention

`completed = n` means the half-open prefix `[0, n)`. There is no inclusive endpoint in this schema. A partial checkpoint for a positive known length must end before that length; only a verified `ready` checkpoint can claim the entire file.

The original design's `completed_ranges` example was illustrative and used inclusive bounds. Neither implemented version imports that format. Version 1 keeps its prefix count and checksum unchanged; it is never automatically converted into range state.

## Version 2 manifest

Version 2 records disjoint whole chunks. This small arithmetic example records `abc` and `ghi` from `abcdefghij`:

```json
{
  "schema_version": 2,
  "destination": "/home/user/Downloads/file",
  "source_id": "18c6f551ea3482332a3ecb8399217217eb214fd68dab3c888af753a504929c82",
  "effective_id": "18c6f551ea3482332a3ecb8399217217eb214fd68dab3c888af753a504929c82",
  "length": 10,
  "etag": "\"version-1\"",
  "encoding": "identity",
  "chunk_size": 3,
  "completed_ranges": [
    {
      "start": 0,
      "end": 3,
      "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
    },
    {
      "start": 6,
      "end": 9,
      "sha256": "50ae61e841fac4e8f9e40baf2ad36ec868922ea48368c18f9535e47db56dd7fb"
    }
  ],
  "sha256": "",
  "state": "partial"
}
```

The identity and destination fields retain their version-1 meanings. Version 2 requires a known nonnegative length, identity encoding, and a strong ETag. It replaces the `completed` field with `chunk_size` and `completed_ranges`; a mixed schema is invalid. Unknown versions are refused.

`chunk_size` is a positive integer that defines at most 256 chunks. New plans choose at least four MiB, increasing the size for large files. Resume retains the saved size. Each array entry contains exactly `start`, exclusive `end`, and the SHA-256 digest of those bytes. Entries must match complete planned chunks, be sorted without duplicates or overlaps, and remain separate when adjacent. The last chunk can be shorter. An empty array is valid; null fields or array elements are not.

Saved-byte totals are derived from validated coverage. The example has six saved bytes and missing chunks `[3, 6)` and `[9, 10)`; six is not a resume offset. A `partial` manifest has an empty top-level `sha256`, even if all chunks have completed. A `ready` manifest additionally requires exact coverage, exact file length, and a full-file SHA-256 digest. Empty ready files use an empty chunk array and the SHA-256 digest of empty content.

`inspect` understands both versions. Fresh ranged downloads create version 2; direct/fallback streams create version 1. Either format resumes automatically after local and remote validation, or explicitly with `--resume`. Saved chunk sizes remain fixed even when the connection limit changes. Both formats can recover verified `ready` publication locally.

## Checkpoint ordering

Version-1 streams checkpoint after four additional MiB or one second of arriving data, whichever happens first. Normal shutdown forces a checkpoint where safe.

1. Write received bytes and update the incremental prefix digest.
2. Sync the data file before advancing saved progress.
3. Write and sync a uniquely named temporary manifest.
4. Publish the manifest atomically and sync the directory.
5. Remove the temporary name when possible.

The final bytes of a known-length response are not recorded as full coverage until the application verifies response completion. A read failure after those bytes therefore leaves an earlier checkpoint, forcing conservative re-fetching.

Unacknowledged data can extend beyond the checkpoint after a crash. Recovery verifies the saved prefix hash and bounds, and does not truncate that tail until the application confirms a compatible remote response. A rejected resume leaves data and metadata unchanged.

For version 2, `ClaimRange` grants one writer an incomplete whole chunk. It rejects duplicate ownership and completed chunks. The writer advances only within that chunk using offset writes; an oversized write fails before changing neighboring bytes. Closing a writer discards its claim without acknowledging bytes, allowing a later attempt to replace the entire chunk.

After validating response identity, bounds, exact body length, and clean completion, the caller invokes the writer's `Commit`. Storage serializes commits, syncs and hashes the chunk, then publishes the checkpoint before releasing ownership. `CheckpointRange` remains available for already quiescent chunks and rejects chunks with an active writer. Metadata snapshots are copied under the same lock. Closing or publishing the transfer is refused while any range writer remains active.

Recovery hashes every saved chunk, checks the highest saved endpoint against the actual file, and ignores unacknowledged gaps. Opening state or calling `ContinueRanges` never truncates the partial file. After remote validation, missing chunks must be re-fetched in full. Newly initialized range state is retained even with zero acknowledged bytes. `FinalizeContext` rechecks coverage and each chunk, honors cancellation during hashing and before publication, records a complete checksum, then uses the existing publication protocol. OS sync calls themselves cannot be interrupted by the context.

## Resume requirements

Saved bytes require a valid manifest, the same destination/source identity, an intact partial file, and matching prefix or chunk checksums. An old Milestone A partial without a manifest is refused; its length is not proof of downloaded coverage.

Network resume additionally requires a known length, identity encoding, a usable strong ETag, and a valid compatible response from the server. Unknown-length, encoded, weak-tagged, or unversioned partial transfers are retained for inspection but cannot reuse bytes. Last-Modified alone is not a resume validator in this milestone.

Resource changes, unsupported ranges, changed effective URLs, corrupt state, and ambiguous artifacts are refused. There is no implicit destructive restart. Use another output path or deliberately abandon existing artifacts after confirming no writer owns them.

## Final publication and recovery

After successful transport and length checks, storage syncs the complete file and checks its stored bytes before publishing a `ready` manifest. Sequential transfers require the actual file size and a fresh full-file digest to match the received stream, including unknown-length streams. A failed readback stops further sequential checkpointing and preserves the previous manifest. Range transfers recheck exact coverage, each chunk, and the complete checksum. Only then does storage publish the final output. Sequential verification adds a full local read before publication and honors cancellation while hashing.

Default publication uses a hard link that cannot replace an existing destination. Explicit overwrite uses rename. Directory sync occurs before removing the partial name; the manifest is removed last. Errors retain whether verified output was already published.

| Recovery state | Behavior |
|---|---|
| Version-1 `partial` manifest and valid prefix | Candidate for validated network resume |
| Version-2 `partial` manifest and valid chunks | Validate the stored representation remotely, then re-fetch whole missing chunks |
| `ready` manifest and intact complete partial | Can finish publication without downloading again |
| `ready` with final/partial names for the same inode | Reconcile completed hard-link publication and cleanup |
| `ready` with no partial and a matching complete final file | Verify full checksum and finish cleanup after rename |
| Changed/missing bytes, unsupported metadata, or unrelated destination | Refuse; preserve evidence and existing files |

Local `ready` recovery does not infer success from a `416` response or file length. It requires the previously recorded complete checksum and matching request identity. It can complete an already received resource version even if the server is now unavailable.

A persistent lock remains after success; partial and manifest files are removed. Completed-download history and automatic stale-artifact cleanup are not implemented.
