# Project status

Updated: October 20, 2026 (America/Sao_Paulo).

## Current state

The [W1 contract](docs/local-web.md) is being implemented. **W1.1a is partial:** the pure metadata model and field/reference validation are implemented; snapshot update rules and the strict codec are next.

No web command, HTTP endpoint, app store, frontend, or frontend dependency is implemented. Existing standalone/Linux daemon behavior is unchanged. Work stays local/no-push; release preparation is outside this build queue.

## Handoff

- [W1 local web](docs/local-web.md#current-implementation) records the current implementation. `internal/app/model.go` defines profiles, defaults, URL-free submission data, and snapshots. `NewSnapshot` takes caller-supplied IDs/state path; `Snapshot.Validate` performs pure validation without creating directories or invoking the daemon. Strict decoding, encoded-size enforcement, and cross-snapshot preservation are not implemented yet.
- The planned web process serves built Vue 3 + TypeScript/Vite assets and a bearer-authenticated IPv4-loopback API. It attaches to explicit daemon startup; web/tab shutdown never stops daemon work. Node/assets remain optional for existing Go builds.
- The planned private `<daemon-state>/web/app.json` and `app.lock` own stable instance identity, profile defaults, and URL-free submission bindings. Default starts with no output directory. Use bounded strict JSON, decimal-string 64-bit values, owned snapshots, atomic saves, and conservative fail-stop; never edit queue/manifest files.
- Persist a URL-free binding before one daemon Add; never replay it. Freeze profile defaults/overrides, keep CLI jobs Unassigned, and retain unresolved IDs after lost replies. Forget/dismiss preserves transfer files. Reuse `transfer.Normalize`, `queue.NewJob`, `daemon.Resolve`/`Call`, safe views, and existing lifecycle rules.
- [HTTP operations](docs/local-web.md#http-interface) use bounded safe polling and existing controls, explicit stale/unavailable views, fixed errors, and no automatic mutation retries. C10 and the W1 parts of C08/C11/S09 are specified; speed/ETA, shared profile budgets, richer controls, and library/history remain later work.
- The owner requested smaller commits. [Delivery steps](docs/local-web.md#delivery-order) separate model/codec, ownership/loading, atomic persistence, HTTP protection, read adapters, CLI/assets, frontend, admission, and controls. A milestone is not a commit boundary; each focused commit includes its tests/docs and handoff update.
- Vue Flow remains selected for W4. Firefox manual handoff/opt-in interception (L1) and hosted targeting (L2) stay later. Preserve **Send this link to Yoink** and **Send download to → Home server → Pato**; instance and profile are separate identities. The TUI remains canceled.

## Validation

- `gofmt -w internal/app/model.go internal/app/model_test.go` and `go test ./internal/app -count=1` passed. Model tests cover explicit initialization without filesystem work, invalid IDs/paths/names/settings, collection limits, reference checks, and historical settings after profile edits.
- `make check` passed formatting, vet, all tests, race tests, and build. The initial sandbox attempt stopped at a read-only Go cache; the permitted rerun passed. Log: `/tmp/yoink-w1-model-check.log`.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` and `git diff --check` passed. Staged whitespace/diffs are reviewed before commit, including new files. Frontend/browser and filesystem-store tests are not applicable yet; no new platform/runtime support is claimed.

## Next steps

1. Finish W1.1a in focused commits: first owned snapshot copies and cross-snapshot identity/revision/submission preservation; then strict JSON field/null/Unicode/integer handling and the 4 MiB encoding limit. Keep filesystem, HTTP, and frontend work separate.
2. Continue W1.1b ownership/loading, then W1.1c atomic persistence and fault recovery as separate commits; preserve the model contract and unrelated files.
3. Follow W1.2a onward for bounded/authenticated HTTP, safe read views, explicit CLI/assets, and the frontend before enabling profile/admission/control workflows. Keep unfinished features disabled and refresh this handoff after each implementation.
4. Follow W2–W4 after their prerequisites; retain Firefox/hosted tracks L1/L2 as later work.

## Open decisions and blockers

No product decision blocks the remaining W1.1a work. Choose frontend/tool pins during scaffolding; optional distribution packaging stays deferred. General TOML/cross-platform config discovery, W2+ metadata migrations/library retention, richer telemetry/profile budgets, browser credentials, scheduling, Firefox bridging, and hosted authentication remain separate later contracts.
