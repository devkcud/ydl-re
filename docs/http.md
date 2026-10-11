# HTTP handling

[Documentation index](README.md)

Parallel downloads, sequential streaming, and safe resume for both checkpoint formats are implemented. Fresh transfers probe range capability when more than one connection is requested; saved range state always revalidates its recorded identity. Automatic retries remain planned.

## Implemented sequential transfer

Fresh transfers with `--connections 1` send a full `GET`, accept only `200`, use a 64 KiB buffer, check known body lengths, and require clean completion for unknown lengths. Probe fallback uses the same receiver. The engine requests identity encoding and disables transparent decompression; if the server still sends encoded content, the exact served bytes are saved and counted without decoding.

The default timeout is 30 seconds per connection, TLS handshake, response-header wait, or idle body read. It is not an overall download deadline: active transfers may run longer. Cancellation closes active requests and retains nonempty partial data without publishing it.

Redirect chains allow at most ten requests. HTTPS-to-HTTP downgrades and URL credentials are rejected. Sensitive headers and referrers are stripped across origins, including subsequent redirects away from the original origin. Request errors avoid exposing URL tokens.

Response headers establish the initial manifest identity. There is no separate HEAD dependency and no automatic retry. Local-server tests cover fresh transfers, resumed suffixes, crash recovery, and incompatible responses.

## Implemented resume validation

Before sending a version-1 resume request, the engine validates the manifest, destination, saved-prefix checksum, and original URL fingerprint. Unknown-length, encoded, unversioned, or weak-tagged partials are refused. Strong ETags must be quoted, valid UTF-8, at most 4096 bytes, and contain no invalid opaque-tag characters.

For a positive saved prefix `n`, send `Range: bytes=n-` and `If-Range` with the saved strong ETag. Require `206`, one exact `Content-Range` covering `n` through `length - 1`, the same total, the same echoed strong ETag, identity encoding, and the same effective-URL fingerprint. A declared payload length must equal the remaining bytes; chunked framing is allowed when actual bytes match those bounds.

For a zero-byte checkpoint, send a full GET with `If-Match` and require matching `200` metadata. Only after successful validation may storage truncate uncheckpointed tail bytes and receive the response.

A `200` received for a positive-offset resume is refused. A `416` triggers a fresh GET re-probe whose body is discarded, then fails without inferring completion. Neither case discards existing state or silently restarts from zero.

An already verified `ready` checkpoint can finish publication locally after source-identity and full-file checksum validation. It does not need a network request and may complete an older, already received resource version while the server is unavailable.

## Parallel checkpoint resume

Storage first validates every saved chunk and its bounds. The original URL must match before network access. For a nonempty representation, `range_resume.go` sends `Range: bytes=0-0` and `If-Range` with the stored strong ETag. It requires the saved effective URL, ETag, total, identity encoding, exact one-byte bounds, and one byte followed by clean completion. That byte is discarded and excluded from progress.

Only after this validation does storage activate range writers. The pool keeps the saved chunk size, subtracts completed chunks, and re-fetches each missing chunk in full. Saved-byte totals never become request offsets. Changing the connection limit affects concurrency, including one-worker resume, without changing the checkpoint format or boundaries. Every subsequent response must still match the stored identity.

Refused validation, timeout, or cancellation before workers start leaves data and metadata unchanged. Ignored ranges never trigger a full-stream fallback for saved state; `416` is re-probed and refused. A later interrupted worker can leave unconfirmed bytes in its missing chunk, but only validated whole chunks advance coverage. Another invocation replaces incomplete chunks in full.

A `partial` checkpoint with all chunks saved still needs remote validation before final verification/publication. A zero-length range checkpoint instead uses a full GET with `If-Match`, matching `200` metadata, an explicit zero length, no `Content-Range`, and clean empty-body completion. Verified `ready` checkpoints retain their offline recovery path.

## Selected range identity

The HTTP layer has a value snapshot for an eligible ranged representation: original/effective URL fingerprints, positive total length, and one strong ETag under identity encoding. It retains no mutable response headers or URL pointers. Creating metadata for storage returns a separate value.

Conditional range requests must use the selected source identity and stay within the total length. They send identity encoding and `If-Range` with the selected ETag. Response validation requires `206`, the same ETag and effective URL, identity encoding, exact assigned bounds/total, and a matching declared payload length when present. The range-bound validator is shared with version-1 resume; zero-prefix resume still follows its existing full-response path.

These identity helpers validate response headers. The capability probe separately verifies its actual body; workers verify their whole assigned bodies and clean completion before checkpointing.

## Fresh capability probing

`internal/transfer/probe.go` sends `GET` with `Range: bytes=0-0`, identity encoding, and the existing redirect/transport policy. It does not use HEAD or trust `Accept-Ranges`. For a `206`, require one exact `Content-Range` for byte zero, a positive known total or an explicit unknown total, a one-byte payload length when declared, and exactly one byte followed by clean response completion. The application reads at most two bytes to detect an oversized body.

| Evidence | Probe result |
|---|---|
| Valid one-byte `206`, known total, identity encoding, one strong ETag | Selected range identity; the probe body is closed and its byte discarded |
| `200` without `Content-Range`, including encoded or unknown-length content | Return that untouched full-response stream for sequential transfer |
| Valid `206` with an unknown total, unsuitable encoding, or unusable validator | Discard the probe, then issue one unconditional full GET |
| `416` | Discard the response, then issue one full GET; never infer an empty or completed file |
| Invalid bounds, wrong/short/oversized body, failed body completion, or another status | Fail without fallback or retry |

The second full GET must return `200` without `Content-Range`. Its metadata and bytes establish a new sequential transfer; no probe byte or identity is reused. It may therefore serve a newer representation without mixing versions. A second unsupported response fails. This is bounded mode selection, not an automatic retry policy; each request still follows the existing redirect limit.

The result contains either the selected representation or an open full-response stream. The coordinator must consume or close that stream, check its body completion and length, and publish only through storage. Probe and returned-stream body reads honor cancellation and the per-read inactivity timeout. A successful probe never authorizes skipping later range validation.

The downloader invokes this helper only for fresh transfers, after acquiring output ownership and before accepting downloaded bytes or initializing metadata. A selected range identity initializes version-2 state and the bounded pool; a returned stream uses version-1 sequential storage. Existing version-1 requests and saved-state refusal remain unchanged.

## Request and response example

Resume a 4,096-byte object after a verified 1,024-byte prefix:

```http
GET /archive.zip HTTP/1.1
Host: example.org
Accept-Encoding: identity
Range: bytes=1024-
If-Range: "version-1"
```

An expected response header set:

```http
HTTP/1.1 206 Partial Content
Content-Range: bytes 1024-4095/4096
Content-Length: 3072
ETag: "version-1"
```

The body is omitted from the example. The engine still needs exactly 3,072 valid body bytes and successful transport completion.

The saved prefix is `[0, 1024)` and the missing interval is `[1024, 4096)`. HTTP reports an inclusive last byte of `4095`. See [the manifest convention](resume-and-storage.md#interval-convention).

## Validate every range

Before marking a worker's interval complete:

1. Require the intended partial response status.
2. Check that `Content-Range` matches the assigned start, end, and known total.
3. Check representation identity against the transfer's selected version.
4. Reject encoding that invalidates byte-offset assumptions.
5. Verify the exact body length and handle every read/write error.
6. Acknowledge only bytes that meet the checkpoint policy.

Never let an oversized response write into a neighboring worker's interval. A short body must leave the interval incomplete. A probe's correct response does not exempt later responses from these checks.

## Full responses and unsatisfied ranges

A `200` received for a range request is not the requested interval. It may mean the server ignored ranges or a conditional range no longer matches. Do not append it at the requested offset.

Resume and active range workers refuse such a response and preserve state. The fresh capability probe can hand an untouched `200` to a sequential receiver before accepting any range bytes. Once workers start, failures cancel the pool without automatic fallback or destructive restart.

A `416` requires re-probing and checking identity, size, and local coverage. It must not be treated as proof that the local file is complete. Empty resources and offsets at the end of an object need explicit tests.

## Representation validators

HTTP permits combining partial responses when they share a strong validator. A weak ETag cannot be sent in `If-Range`. A date is eligible for `If-Range` only without an ETag and when it qualifies as a strong validator; merely having `Last-Modified` is insufficient. [RFC 9110: combining parts](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.7.3), [If-Range](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5).

The implementation applies the resource-version invariant to sequential resume and every parallel response. Equal URLs and lengths alone cannot prove that responses contain the same version.

Fresh streams may work without validators, but partial reuse requires a usable strong ETag in both stored metadata and the new response. Last-Modified is not used as a resume validator. Changed URLs or versions and unsupported range behavior are refused rather than silently restarted.

## Encoding and unknown lengths

For byte-range transfers, request `Accept-Encoding: identity` and disable transparent decompression. Go's transport can automatically request gzip and decode it; `DisableCompression` controls that automatic behavior. An explicitly unexpected content encoding still requires validation. [Go HTTP transport](https://pkg.go.dev/net/http#Transport).

Compressed responses and unknown-size bodies use a sequential plan. The current engine saves encoded bytes unchanged and checks lengths against those same bytes. Do not compare a decoded byte count against a size describing encoded bytes.

Without a reliable total, report bytes and speed. Successful EOF must mean the HTTP body completed without a transport error. A clean EOF with no expected length cannot independently prove the publisher's intended object size.

## Status and failure handling

| Response or failure | Policy |
|---|---|
| `200` for a fresh full transfer | Stream and verify completion |
| Valid `206` for an assigned interval | Receive and verify that interval |
| `200` for an interval | Refuse active ranges; the fresh probe can select an untouched full stream |
| `416` | Re-probe; never infer completion from status alone |
| `408`, `429`, selected `5xx`, transient network failures | Fail currently; bounded retry policy is the next scheduling feature |
| Permanent `404` or other non-retryable failure | Fail clearly |
| Bad range, mismatched validator, wrong length | Reject unsafe bytes and preserve only valid progress |

Do not retry all `4xx` or all `5xx` indiscriminately. `Retry-After` handling and bounded backoff belong to the [scheduler](concurrency.md#retries).

## Redirects, proxies, and timeouts

Bound redirects and track the effective URL. Reassess resource identity when redirects change. Do not forward credentials or cookies indiscriminately to another origin.

Respect proxy and certificate verification defaults. The current network timeout defaults to 30 seconds for connection, TLS, response headers, and each idle body read; it does not cap total download duration.

See [security](security.md) for credential handling and [testing](testing.md) for the HTTP fault matrix.
