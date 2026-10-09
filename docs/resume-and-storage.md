# Resume and storage

[Documentation index](README.md)

v0.1 is designed to recover transfers from local files without a daemon. This page describes the proposed storage model; filenames and schema are not a released compatibility contract.

## Files and ownership

For a destination named `archive.zip`, the design illustrates:

```text
archive.zip.yoinkdl.part
archive.zip.yoinkdl.json
```

The partial file contains transfer bytes. The JSON manifest records identity and reusable progress. The final destination is published only after verification.

Place partial data on the destination's filesystem so finalization does not require a cross-filesystem copy. Transfer sidecars intentionally live outside the central configuration directory.

One process may write a transfer at a time. Acquire an exclusive lock or equivalent ownership mechanism before creating, recovering, truncating, or publishing its files. The lock filename, primitive, and stale-owner recovery policy are undecided.

## Illustrative manifest

This preserves the example convention from the original design:

```json
{
  "schema_version": 1,
  "source_url": "https://example.org/archive.zip",
  "effective_url": "https://cdn.example.org/archive.zip",
  "destination": "/home/user/Downloads/archive.zip",
  "length": 104857600,
  "etag": "\"example-validator\"",
  "last_modified": null,
  "mode": "ranged",
  "completed_ranges": [[0, 16777215], [33554432, 50331647]],
  "state": "interrupted"
}
```

| Field | Intended meaning |
|---|---|
| `schema_version` | Identifies the manifest interpretation |
| `source_url` | Original resource locator |
| `effective_url` | Resource locator after redirects |
| `destination` | Intended final local path |
| `length` | Reliable total representation size, when available |
| `etag` | Recorded ETag; eligibility still needs validation |
| `last_modified` | Recorded modification date, if present |
| `mode` | Selected transfer mode |
| `completed_ranges` | Intervals safe to consider complete under this schema |
| `state` | Last recorded lifecycle state |

Nullability, enums, unknown-length encoding, required fields, migrations, and malformed-field behavior are not specified by this example. The URL values contain no credentials; a real persistence policy must reconcile resource identity with secret redaction.

Treat the source identity, destination, representation version, and interval convention as fixed for a compatible transfer. Changing these requires a new transfer or an explicitly validated transition, not silent manifest edits.

## Interval convention

The JSON example uses **inclusive endpoints**. `[0, 16777215]` contains 16,777,216 bytes. Its two saved intervals cover 33,554,432 bytes of a 104,857,600-byte object, with gaps still to download.

The design recommends internal half-open intervals `[start, end)`. For example, `[0, 16777216)` represents the same first interval, with length `end - start`.

Before shipping, choose and document one on-disk convention. Never read the sample's inclusive endpoints as half-open endpoints. Never change a persisted convention without a versioned migration or a safe rejection policy. HTTP range conversion belongs at the transport boundary.

## Conditions for safe resume

Before reusing data, validate:

1. The manifest can be read and its schema is understood.
2. It identifies the intended destination and compatible source representation.
3. The partial file exists and is usable under the ownership and permission policy.
4. Every recorded interval is in bounds and coverage contains no invalid overlaps.
5. The server still supports the requests needed to retrieve missing intervals safely.
6. Saved bytes satisfy the checkpoint's durability assumptions.

A filename, file size, or stored state of `interrupted` is insufficient. A preallocated file may contain holes. A matching URL may serve new content. A weak or missing validator may prevent reuse.

Refuse unsafe reuse. Preserve incompatible state until a deliberate restart policy applies. `--overwrite` addresses a completed destination and does not establish partial-data compatibility.

## Checkpoint ordering

The manifest is a recovery checkpoint, not a byte-by-byte activity log. It may lag behind the live progress display.

A recommended flush-before-checkpoint sequence is:

1. Finish and validate an interval, then mark it eligible for checkpointing.
2. Make eligible file writes durable under the selected platform policy.
3. Snapshot only completed intervals covered by that durability boundary.
4. Write the next manifest to a temporary file beside the current one.
5. Sync and publish the manifest using the qualified atomic replacement strategy.
6. Apply directory durability operations where required by the platform policy.

The design also allows conservatively re-fetching uncertain bytes after restart. Whichever approach is chosen, a new manifest must not authorize trusting data that did not survive with it.

Checkpoint frequency, batching, file permissions, temporary naming, and platform durability details are open decisions. Do not sync on every byte or assume a successful buffered write proves persistence.

## Recovery cases

| Observed artifacts | Safe interpretation |
|---|---|
| Matching valid manifest and partial file | Candidate for resume after remote and local validation |
| Partial file without a manifest | Bytes have no trustworthy recorded identity/coverage; do not silently resume |
| Manifest without a partial file | Recorded progress cannot be reused |
| Corrupt or unsupported manifest | Refuse unsafe recovery; preserve evidence |
| Data written after the last checkpoint | Re-fetch any portion recovery cannot prove safe |
| Completed output with leftover metadata | Reconcile finalization; do not truncate or resume the completed file automatically |
| Lock apparently held by another process | Refuse a second writer until ownership is resolved safely |

The exact user interaction for each incompatible case is undecided. A separate fresh output path allows a new transfer without altering the old artifacts.

## Interrupted writes and retries

A failed interval must remain incomplete. The simplest proposed implementation re-fetches that interval and overwrites only its owned region. More granular sub-interval recovery needs its own validation and checkpoint rules.

Retrying must not append a full response to an offset, overlap another worker, or mark an interval complete before its response and writes have been validated.

## Final publication and cleanup

Sync and close verified data before publication. Preserve the destination collision policy even if another file appears during the transfer. Remove metadata only after the final output is safely established.

Crash recovery must handle a rename that succeeded before metadata cleanup. The cleanup outcome and final-result policy need explicit tests. See [finalization](transfer-lifecycle.md#7-finalize).

There is no v0.1 cleanup command specified. Manual removal of sidecars discards recovery information and should only follow a deliberate decision to abandon a transfer, with no writer active. Do not manually rename a partial file to make it look completed.
