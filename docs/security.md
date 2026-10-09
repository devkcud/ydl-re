# Security and file safety

[Documentation index](README.md)

Milestone A tests cover artifact preservation, exclusive ownership, destination collisions, unsafe paths, redirects, and secret-safe errors. These requirements also guide the remaining v0.1 security review; they do not imply a completed security audit.

## Trust boundaries

The standalone CLI accepts a URL and filesystem destination from its user. Remote servers supply redirects, headers, body bytes, and filenames that must be treated as untrusted. Local manifests are recovery input and also need validation before they authorize writes or reuse.

The engine must protect completed files, keep partial data from being mistaken for success, and avoid leaking secrets while reporting failures.

## Files and paths

- Preserve a completed destination unless the user explicitly chooses `--overwrite`.
- Keep incomplete bytes in a separate partial file until verified publication.
- Sanitize URL-derived filenames, including traversal attempts and special platform filenames.
- Prefer an explicit `--output` over server-supplied filename suggestions.
- Enforce exclusive ownership of sidecars and validate metadata before using saved paths or intervals.
- Use restricted state permissions and safe creation operations appropriate to the platform.

The implementation must define how it handles symlinks, destination races, existing sidecars, missing parent directories, and path normalization. A read-time destination check alone is not a complete publication policy.

Check available disk space when practical, while still handling write-time exhaustion. On `ENOSPC`, permissions errors, or failed syncs, do not claim success or delete otherwise valid partial data automatically.

## Network behavior

Keep certificate verification enabled. Respect normal proxy configuration. v0.1 does not specify an insecure-TLS bypass flag.

Bound redirects and review cross-origin credential handling. Do not forward authorization headers or cookies indiscriminately. A changed effective URL also affects transfer identity and recovery decisions.

Validate range responses, byte counts, representation identity, and encoding. A successful status or stable length does not make mismatched resource versions safe to combine.

## Credentials and local state

URLs may contain user information, signed query parameters, or other access tokens. Avoid exposing those values in progress, errors, logs, JSON output, or shareable diagnostics.

The sample manifest includes source/effective URLs to explain identity. It is not a decision to persist credential-bearing URLs verbatim. The storage design must decide what it needs for safe recovery, what to redact, and whether sensitive data needs a separate protected representation.

Do not assume stripping every query parameter preserves resource identity. Redacted display and exact transfer matching serve different purposes and need a deliberate policy.

Persistent logs, credential storage, custom authentication flags, and secret-management integrations are not specified for v0.1.

## Downloaded content

Never execute downloaded files or scripts automatically. Automatic unpacking and completion scripts are outside v0.1.

Length and range checks establish transfer consistency under the protocol assumptions. They are not malware scanning, publisher authentication, or a user-supplied cryptographic checksum. User checksum verification is deferred; fixture hashes remain useful in tests.

## Future daemon and hooks

Before any daemon accepts remote URLs from other clients, define authentication, permissions, request ownership, and handling of loopback/private destinations and redirects. A network-exposed API would let remote callers initiate requests from the daemon's network and needs explicit SSRF defenses.

No fixed listening port or public HTTP API is committed. A local IPC transport still needs appropriate access controls.

Optional v0.3 hooks require explicit enablement and a separate execution/security design. They must not make arbitrary downloaded content executable by default. Scripting runtimes and sandboxed workflows remain unscheduled.

## Review evidence before release

The [test plan](testing.md) must cover destination collisions, simultaneous writers, malformed state, changed representations, credential-safe redirects, path safety, and failures during checkpointing/finalization.

The project's license, security contact, and disclosure process have not been selected. Do not invent a contact address or claim a published policy until those exist.
