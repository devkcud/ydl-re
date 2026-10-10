# HTTP handling

[Documentation index](README.md)

Sequential HTTP(S) downloads and safe sequential resume are implemented. Parallel ranges and automatic retries remain planned.

## Implemented sequential transfer

Fresh transfers send a full `GET`, accept only `200`, use a 64 KiB buffer, check known body lengths, and require clean completion for unknown lengths. The engine requests identity encoding and disables transparent decompression; if the server still sends encoded content, the exact served bytes are saved and counted without decoding.

The default timeout is 30 seconds per connection, TLS handshake, response-header wait, or idle body read. It is not an overall download deadline: active transfers may run longer. Cancellation closes active requests and retains nonempty partial data without publishing it.

Redirect chains allow at most ten requests. HTTPS-to-HTTP downgrades and URL credentials are rejected. Sensitive headers and referrers are stripped across origins, including subsequent redirects away from the original origin. Request errors avoid exposing URL tokens.

Response headers establish the initial manifest identity. There is no separate HEAD dependency and no automatic retry. Local-server tests cover fresh transfers, resumed suffixes, crash recovery, and incompatible responses.

## Implemented resume validation

Before sending a resume request, the engine validates the manifest, destination, saved-prefix checksum, and original URL fingerprint. Unknown-length, encoded, unversioned, or weak-tagged partials are refused. Strong ETags must be quoted, valid UTF-8, at most 4096 bytes, and contain no invalid opaque-tag characters.

For a positive saved prefix `n`, send `Range: bytes=n-` and `If-Range` with the saved strong ETag. Require `206`, one exact `Content-Range` covering `n` through `length - 1`, the same total, the same echoed strong ETag, identity encoding, and the same effective-URL fingerprint. A declared payload length must equal the remaining bytes; chunked framing is allowed when actual bytes match those bounds.

For a zero-byte checkpoint, send a full GET with `If-Match` and require matching `200` metadata. Only after successful validation may storage truncate uncheckpointed tail bytes and receive the response.

A `200` received for a positive-offset resume is refused. A `416` triggers a fresh GET re-probe whose body is discarded, then fails without inferring completion. Neither case discards existing state or silently restarts from zero.

An already verified `ready` checkpoint can finish publication locally after source-identity and full-file checksum validation. It does not need a network request and may complete an older, already received resource version while the server is unavailable.

## Planned parallel mode selection

`HEAD` can supply metadata, but some servers reject it or return misleading headers. Use a ranged `GET` when necessary to validate actual behavior. Record the effective URL after redirects, a reliable total length if known, and eligible validators.

| Evidence | Intended plan |
|---|---|
| Valid range response, reliable total, identity encoding, compatible representation identity | Bounded ranged download |
| Range request ignored with a full response | Sequential fallback after safely abandoning the range plan |
| Unknown total length | One sequential stream |
| Content encoding unsuitable for byte offsets | One sequential stream |
| Inconsistent or malformed responses | Reject the unsafe result; retry or restart only under an explicit safe policy |

`Accept-Ranges: bytes` is a hint, not proof. Server behavior must remain valid on every worker request, even after a successful probe.

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

The current resume path refuses such a response and preserves state. Future parallel fallback must cancel outstanding work and reconcile representation identity before starting a separate sequential plan; it cannot merely change a worker's write offset.

A `416` requires re-probing and checking identity, size, and local coverage. It must not be treated as proof that the local file is complete. Empty resources and offsets at the end of an object need explicit tests.

## Representation validators

HTTP permits combining partial responses when they share a strong validator. A weak ETag cannot be sent in `If-Range`. A date is eligible for `If-Range` only without an ETag and when it qualifies as a strong validator; merely having `Last-Modified` is insufficient. [RFC 9110: combining parts](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.7.3), [If-Range](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5).

The implementation applies the resource-version invariant to resumed transfers. The future parallel engine must preserve it too. Equal URLs and lengths alone cannot prove that responses contain the same version.

Fresh streams may work without validators, but partial reuse requires a usable strong ETag in both stored metadata and the new response. Last-Modified is not used as a resume validator. Changed URLs or versions and unsupported range behavior are refused rather than silently restarted.

## Encoding and unknown lengths

For byte-range transfers, request `Accept-Encoding: identity` and disable transparent decompression. Go's transport can automatically request gzip and decode it; `DisableCompression` controls that automatic behavior. An explicitly unexpected content encoding still requires validation. [Go HTTP transport](https://pkg.go.dev/net/http#Transport).

Compressed responses and unknown-size bodies use a sequential plan. The current engine saves encoded bytes unchanged and checks lengths against those same bytes. Do not compare a decoded byte count against a size describing encoded bytes.

Without a reliable total, report bytes and speed. Successful EOF must mean the HTTP body completed without a transport error. A clean EOF with no expected length cannot independently prove the publisher's intended object size.

## Status and failure handling

| Response or failure | Planned action |
|---|---|
| `200` for a fresh full transfer | Stream and verify completion |
| Valid `206` for an assigned interval | Receive and verify that interval |
| `200` for an interval | Stop unsafe range handling; consider safe sequential fallback |
| `416` | Re-probe; never infer completion from status alone |
| `408`, `429`, selected `5xx`, transient network failures | Apply bounded retry policy |
| Permanent `404` or other non-retryable failure | Fail clearly |
| Bad range, mismatched validator, wrong length | Reject unsafe bytes and preserve only valid progress |

Do not retry all `4xx` or all `5xx` indiscriminately. `Retry-After` handling and bounded backoff belong to the [scheduler](concurrency.md#retries).

## Redirects, proxies, and timeouts

Bound redirects and track the effective URL. Reassess resource identity when redirects change. Do not forward credentials or cookies indiscriminately to another origin.

Respect proxy and certificate verification defaults. The current network timeout defaults to 30 seconds for connection, TLS, response headers, and each idle body read; it does not cap total download duration.

See [security](security.md) for credential handling and [testing](testing.md) for the HTTP fault matrix.
