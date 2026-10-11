# Security and file safety

[Documentation index](README.md)

The v0.1 standalone CLI has completed a local code review of URL/header/manifest input, diagnostics, filenames, artifact ownership, and failure preservation. Regression fixes and the full Go gate have Linux evidence. This is not an independent audit or release certification; native platform, filesystem durability, and disclosure-policy work remain.

## Trust boundaries

The standalone CLI accepts a URL and filesystem destination from its user. Servers supply untrusted redirects, headers, and bytes. URL-derived basenames and local manifests are also untrusted input. A manifest never chooses a different output directory or authorizes writes outside the caller's destination and validated intervals.

The caller, process environment, and output directory are trusted. Use a directory whose contents other users cannot replace, and avoid external modification of transfer artifacts while a transfer is active. Root or another process running as the same user can defeat filesystem checks between operations. Advisory locks coordinate participating YoinkDL writers; they do not prevent arbitrary external writes. Checksums establish consistency, not authenticity of a manifest someone can rewrite.

## Files and paths

- Resolve an explicit output to an absolute path and require its parent to exist. Operations are relative to the opened parent directory. Parent symlinks can resolve to the caller-selected directory; destination and sidecar symlinks/special files are refused.
- Without `--output`, decode only the original URL's final path component. Replace path separators, reserved punctuation, and nonprinting Unicode characters with `_`, trim surrounding spaces/dots, and fall back to `download` for empty names or names over 200 bytes. Printable localized names remain supported. Known device basenames are prefixed; transfer artifact suffixes and temporary-name segments cannot become ordinary outputs. `Content-Disposition` is ignored.
- New partials, manifests, temporary checkpoints, and locks use private `0600` permissions. Mutable partials require one hard link and no group/other permission bits, checked during recovery and before writes. Reject incompatible state without chmod, truncation, or alias deletion. `--overwrite` does not override these checks.
- Keep a persistent advisory-lock inode and exclusive chunk claims. Never delete a lock file to guess whether an owner died. Data sync precedes atomic metadata publication; random temporary names use exclusive creation.
- Preserve completed destinations by default, including a destination that appears during transfer. Publish by a non-overwriting hard link, or rename with explicit `--overwrite`. Check actual file identity at publication. Verified `ready` recovery may reconcile the intentional final/partial links from interrupted publication without writing content bytes.

The manifest loader limits input to 64 KiB, validates UTF-8, requires exact unique/non-null fields, and checks versions, hashes, lengths, state, destination identity, and chunk bounds. Version 2 permits at most 256 sorted disjoint whole chunks. Unsupported or corrupt input is refused without destructive fallback. See [storage contracts](resume-and-storage.md).

Sequential publication re-reads the stored file and compares its exact size and digest with the received stream; range publication verifies coverage and chunk hashes. Failed sequential readback preserves the previous manifest even during close. Storage failures do not authorize publication or automatic deletion of nonempty partials. A sync or cleanup error after publication reports that verified output already exists. Real disk exhaustion and power-loss qualification remain pending.

## Network behavior

TLS certificate verification stays enabled, normal proxy environment configuration is respected, and no insecure-TLS bypass exists. Only absolute HTTP(S) URLs without user information are accepted. Redirects allow at most ten requests, refuse HTTPS downgrades, and strip sensitive headers and referrers across origins, including later redirects away from the initial origin.

Full responses require `200` without `Content-Range`, including zero-prefix resume before any uncertain tail is truncated. Ranges require exact bounds, length, identity encoding, effective-URL fingerprint, and strong ETag. Headers are bounded to one MiB per response, validators to 4096 bytes, and malformed bodies are terminal. See [HTTP handling](http.md) for timeouts, retry bounds, and representation validation.

Unknown-length streams still rely on clean transport completion and can consume arbitrary disk space until completion, interruption, or a write failure. There is no download-size quota. The foreground CLI permits private and loopback destinations intentionally: its caller already controls the request. This policy is not suitable for an untrusted remote API.

## Credentials and local state

Source URLs, query parameters, raw response headers, and stored ETags are omitted from normal progress, errors, JSON, and inspection output. Human text escapes nonprinting characters; JSON uses standard string escaping. File paths remain visible.

Invalid option values are omitted from diagnostics. HTTP request and body errors are classified without raw URLs, headers, or parser messages. The private transport closes connections after responses to prevent its idle-connection diagnostic from logging unsolicited server bytes. This adds connection setup costs; explicitly enabled runtime debug tracing is outside this policy. Local paths are still reported, so use an explicit output name when the URL basename itself contains sensitive information.

Both manifest versions store SHA-256 fingerprints of parsed source/effective URLs with fragments removed. They do not store raw URLs, query tokens, or request headers. They retain the strong ETag and local checkpoint fields needed for recovery under private creation permissions; treat state files as private. The original draft's URL-bearing schema was illustrative and is not accepted by the loader.

Query parameters remain part of the hashed identity. A refreshed signed URL or changed redirect target therefore fails matching rather than silently reusing bytes. Fingerprints are identity checks, not encryption or authentication of manifest contents.

Shell history, process argument inspection, caller-enabled tracing, environment secrets, and filesystem access are outside output redaction. A URL basename or explicit local path may itself contain sensitive information. There is no persistent application log, credential store, cookie jar, custom authentication flag, or secret-management integration.

## Downloaded content

Never execute downloaded files or scripts automatically. Automatic unpacking and completion scripts are outside v0.1.

Length and range checks establish transfer consistency under the protocol assumptions. They are not malware scanning, publisher authentication, or a user-supplied cryptographic checksum. User checksum verification is deferred; fixture hashes remain useful in tests.

## Future daemon and hooks

Before any daemon accepts remote URLs from other clients, define authentication, permissions, request ownership, and handling of loopback/private destinations and redirects. A network-exposed API would let remote callers initiate requests from the daemon's network and needs explicit SSRF defenses.

No fixed listening port or public HTTP API is committed. A local IPC transport still needs appropriate access controls.

Optional v0.3 hooks require explicit enablement and a separate execution/security design. They must not make arbitrary downloaded content executable by default. Scripting runtimes and sandboxed workflows remain unscheduled.

## Review evidence

| Boundary | Current proof |
|---|---|
| Invalid arguments | [CLI security tests](../internal/cli/security_test.go) cover scalar and boolean option values carrying synthetic credentials; failure precedes transfer work |
| HTTP input and diagnostics | [HTTP security tests](../internal/transfer/security_test.go) cover malformed trailer redaction, fresh contradictory headers, and unchanged zero-prefix recovery state; [process tests](../cmd/yoinkdl/security_test.go) cover unsolicited transport bytes |
| Names and redirects | [HTTP policy tests](../internal/transfer/http_test.go) cover escaped separators, device/artifact names, invisible/bidirectional characters, printable localized names, and cross-origin credential stripping |
| Mutable file ownership | [Storage security tests](../internal/storage/security_test.go) cover both schemas, pre-existing and late hard links, shared permissions, and preservation on refusal |
| Publication integrity | The same storage tests reject changed, short, or extended sequential bytes, including unknown lengths; [prefix](../internal/storage/checkpoint_test.go) and [range](../internal/storage/range_checkpoint_test.go) tests retain verified link/rename recovery |
| Hostile saved state | [Prefix schema tests](../internal/storage/checkpoint_test.go) and [range schema tests](../internal/storage/range_manifest_test.go) reject malformed, oversized, duplicate, null, incompatible, and invalid-coverage input |

The full `make check` gate covers formatting, vet, tests, race tests, and build. [Testing](testing.md) records broader coverage and limits. Linux results do not qualify native macOS behavior, Windows storage, network filesystems, power-loss durability, or every possible concurrent external filesystem attack.

## Remaining release policy

The project's license, security contact, and disclosure process have not been selected. Do not invent a contact address or claim a published policy until those exist.
