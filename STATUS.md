# Project status

Updated: October 12, 2026.

## Current state

Milestones A, B, and C are implemented and pass the local Linux gate. The CLI supports sequential/parallel transfers, safe resume, exclusive offset writes, verified checkpoints, bounded retries, inspection, cancellation, overwrite protection, and inactivity timeouts. Milestone D includes aggregate body-rate caps through `--limit` and versioned newline-delimited output through `--json`.

Text terminal/error handling, the local security review, and the platform/build evidence are recorded. Linux/amd64 passes the full gate on ext4 and uncached CGO-disabled tests on tmpfs. Six CLI targets build; foreign test programs compile without native execution. Native qualification beyond Linux/amd64, CI, and release tooling remain. Configuration loading is optional later work. A full v0.1 release is not complete.

## Handoff

- [Platform qualification](docs/platforms.md) owns the six-target matrix, exact source/toolchain/filesystem evidence, reproduction commands, and limits. Runtime/test sources remain at `ee2f295`; this qualification required only documentation changes. The full gate used an ext4-backed temporary directory under ignored `bin/`, removed after successful cleanup. The uncached CGO-disabled run used tmpfs.
- CLI builds and foreign test compilation passed for Linux, macOS, and Windows on amd64/arm64 with CGO disabled. Outputs remain temporary validation artifacts under `/tmp/yoinkdl-platform.irzOOg`, not release packages. Only Linux/amd64 ran natively. Linux/arm64/macOS storage is enabled but unqualified; Windows download/inspection storage remains unsupported. No CI, installer, signing, distribution contract, or release tag was added.
- The [security review](docs/security.md) is complete locally. The caller/environment/output directory remain trusted. Invalid option values and raw HTTP parser text are omitted from diagnostics. Full responses require 200 without Content-Range before saved tails can be truncated. Derived names replace nonprinting Unicode while explicit paths stay exact. Local paths remain visible.
- Mutable partials require one hard link and private Unix permissions. Refusal preserves artifacts; verified ready recovery accepts intentional publication links. Sequential finalization re-reads the stored file and matches its size/checksum against received bytes; failed verification prevents later checkpoint advancement. HTTP connection reuse remains disabled to prevent unsolicited-byte logging. These checks add local read and connection setup costs.
- Text/JSON output uses bounded writes and cancellable owned descriptors for Linux/macOS pipes, sockets, and detected terminals. Output failures join workers, release storage, and preserve published data. Invalid invocations use exit 2, other failures use 1, and interruption uses 130. JSON records are versioned; counters are advisory. See [CLI output](docs/cli.md#json-output).
- `--limit` owns one body-byte budget across probes, redirects, workers, and retries. Failed traffic stays charged; saved progress supplies no credit. `--connections` is 1..32 (default four), and `--retries` is 0..10 additional attempts (default three). A chunk shares one request/body budget; identity/protocol/storage failures are terminal. See [concurrency](docs/concurrency.md).
- Version 1 remains a checksummed prefix; version 2 stores disjoint whole chunks and preserves saved chunk boundaries. Local checks and conditional GET validation precede writes; refused validation preserves artifacts. Only verified ready state recovers offline. Keep the persistent advisory-lock inode and explicit completed-file overwrite policy. See [storage](docs/resume-and-storage.md).
- The original design is unchanged. Keep focused local commits, refresh this handoff after each implementation step, and never push.

## Validation

- Native environment: Go `go1.27.2-X:nodwarf5`, Linux/amd64, Arch Linux kernel `7.2.9-zen1-1-zen`, `GOAMD64=v1`, `CGO_ENABLED=1`, GCC `16.2.1`, UID 1000. `findmnt` identifies workspace ext4 and default `/tmp` tmpfs.
- `TMPDIR=/nvme/Projects/personal/yoink-reborn/bin/platform.E9zScW make check` passed on ext4: formatting, vet, ordinary tests, race tests, and build. Only the pure range planner reused its matching cached result. `CGO_ENABLED=0 go test -count=1 ./...` passed uncached on tmpfs. Existing security, recovery, retry, rate, terminal, output, and process suites are included.
- All six `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -p 2 -trimpath -o <output> ./cmd/yoinkdl` checks passed. All five test packages compiled for each of the five foreign targets with `go test -p 2 -c -o <directory>/ ./...` under the same environment. `file` confirmed ELF/Mach-O/PE architectures; `go version -m` records the clean source revision and CGO-disabled build. The static Linux/amd64 CLI's `version` command passed.
- Native Linux/arm64, macOS, and Windows execution/race checks were not run because this session has only a Linux/amd64 host. No network-filesystem, full-disk, or real power-loss qualification was attempted. Compilation is not native support evidence.
- `python3 /tmp/yoinkdl-security-doccheck.py` passed after the matrix updates: 21 Markdown files, 243 local links/anchors, 28 shell blocks, four JSON examples, one TOML example, fences, and whitespace. `git diff --check` passed. `git diff --quiet ee2f295 -- cmd internal go.mod Makefile yoinkdl-design.md` confirmed that runtime/test/build sources and the original design are unchanged.

## Next steps

1. Resolve the release choices with the user: license, distribution channels and intended release targets, and a real security-reporting contact/process. Use [R01/R02/R05](docs/decisions.md#release-and-compatibility-decisions); do not invent ownership or policy details.
2. Implement CI and release tooling for the chosen targets, with validated installation instructions, artifact/version naming, and a changelog. Obtain native evidence before advertising any additional platform. Keep configuration files and daemon/TUI work optional/later.

## Open decisions and blockers

The requested local platform/build qualification is complete within the recorded limits. Release choices remain unresolved; native ARM/macOS/Windows environments are unavailable here. Windows path/storage implementation, broader filesystem durability, native CI, and published support promises remain open in [the decision register](docs/decisions.md).
