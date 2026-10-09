# HTTP handling

[Documentation index](README.md)

The sequential HTTP(S) engine is implemented. Range probing, parallel ranges, and resumption described below remain planned for later milestones.

## Implemented sequential transfer

The engine sends a full `GET`, accepts only `200`, uses a 64 KiB buffer, checks known body lengths, and requires clean completion for unknown lengths. It requests identity encoding and disables transparent decompression; if the server still sends encoded content, the exact served bytes are saved and counted without decoding.

The default timeout is 30 seconds per connection, TLS handshake, response-header wait, or idle body read. It is not an overall download deadline: active transfers may run longer. Cancellation closes active requests and retains nonempty partial data without publishing it.

Redirect chains allow at most ten requests. HTTPS-to-HTTP downgrades and URL credentials are rejected. Sensitive headers and referrers are stripped across origins, including subsequent redirects away from the original origin. Request errors avoid exposing URL tokens.

There are no automatic retries, probes, ranges, or manifests yet. The tests use local servers and verify byte hashes, interruption, truncation, timeout behavior, and publication failures.

## Probe and select a mode

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

An illustrative request for the second 1,024-byte interval of a 4,096-byte object:

```http
GET /archive.zip HTTP/1.1
Host: example.org
Accept-Encoding: identity
Range: bytes=1024-2047
If-Range: "version-1"
```

An expected response header set:

```http
HTTP/1.1 206 Partial Content
Content-Range: bytes 1024-2047/4096
Content-Length: 1024
ETag: "version-1"
```

The body is omitted from the example. The worker still needs exactly 1,024 valid body bytes and successful transport completion.

The internal interval `[1024, 2048)` is half-open; the HTTP range ends at `2047`. Keep that conversion in one place. The illustrative manifest in the original design uses inclusive endpoints; see [the manifest convention](resume-and-storage.md#interval-convention).

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

Safe fallback requires canceling outstanding range work, reconciling the chosen representation and stored progress, and starting a separate sequential plan when the restart policy allows. It is not merely a worker changing its write offset.

A `416` requires re-probing and checking identity, size, and local coverage. It must not be treated as proof that the local file is complete. Empty resources and offsets at the end of an object need explicit tests.

## Representation validators

HTTP permits combining partial responses when they share a strong validator. A weak ETag cannot be sent in `If-Range`. A date is eligible for `If-Range` only without an ETag and when it qualifies as a strong validator; merely having `Last-Modified` is insufficient. [RFC 9110: combining parts](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.7.3), [If-Range](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5).

The implementation must apply the resource-version invariant to both parallel transfers and later resume attempts. Equal URLs and lengths alone cannot prove that two responses contain the same version.

Recommended conservative behavior is one fresh stream when a safe common validator cannot be established, and refusal to reuse saved bytes whose identity is uncertain. The exact validator-selection and restart policies remain [open](decisions.md#http-and-scheduling-decisions).

## Encoding and unknown lengths

For byte-range transfers, request `Accept-Encoding: identity` and disable transparent decompression. Go's transport can automatically request gzip and decode it; `DisableCompression` controls that automatic behavior. An explicitly unexpected content encoding still requires validation. [Go HTTP transport](https://pkg.go.dev/net/http#Transport).

Compressed responses and unknown-size bodies use a sequential plan. The exact treatment of encoded sequential output remains undecided. Do not compare a decoded byte count against a size describing encoded bytes.

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

Respect proxy and certificate verification defaults. A request can be slow without being stalled, so avoid one short overall timeout that kills legitimate large transfers. Exact dial, response, idle, and total timeout policies are not yet defined.

See [security](security.md) for credential handling and [testing](testing.md) for the HTTP fault matrix.
