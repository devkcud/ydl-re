# Project status

Updated: October 20, 2026 (America/Sao_Paulo).

## Current state

The [W1 contract](docs/local-web.md) is complete. **W1.1a, the pure app metadata model and strict codec, is the next implementation step.**

No web command, HTTP endpoint, app store, frontend, or frontend dependency is implemented. Existing standalone/Linux daemon behavior is unchanged. Work stays local/no-push; release preparation is outside this build queue.

## Handoff

- [W1 local web](docs/local-web.md) owns the specified workflow and acceptance cases. It is a specification, not a working command or runtime qualification.
- The planned web process serves built Vue 3 + TypeScript/Vite assets and a bearer-authenticated IPv4-loopback API. It attaches to explicit daemon startup; web/tab shutdown never stops daemon work. Node/assets remain optional for existing Go builds.
- The planned private `<daemon-state>/web/app.json` and `app.lock` own stable instance identity, profile defaults, and URL-free submission bindings. Default starts with no output directory. Use bounded strict JSON, decimal-string 64-bit values, owned snapshots, atomic saves, and conservative fail-stop; never edit queue/manifest files.
- Persist a URL-free binding before one daemon Add; never replay it. Freeze profile defaults/overrides, keep CLI jobs Unassigned, and retain unresolved IDs after lost replies. Forget/dismiss preserves transfer files. Reuse `transfer.Normalize`, `queue.NewJob`, `daemon.Resolve`/`Call`, safe views, and existing lifecycle rules.
- [HTTP operations](docs/local-web.md#http-interface) use bounded safe polling and existing controls, explicit stale/unavailable views, fixed errors, and no automatic mutation retries. C10 and the W1 parts of C08/C11/S09 are specified; speed/ETA, shared profile budgets, richer controls, and library/history remain later work.
- The owner requested smaller commits. [Delivery steps](docs/local-web.md#delivery-order) separate model/codec, ownership/loading, atomic persistence, HTTP protection, read adapters, CLI/assets, frontend, admission, and controls. A milestone is not a commit boundary; each focused commit includes its tests/docs and handoff update.
- Vue Flow remains selected for W4. Firefox manual handoff/opt-in interception (L1) and hosted targeting (L2) stay later. Preserve **Send this link to Yoink** and **Send download to → Home server → Pato**; instance and profile are separate identities. The TUI remains canceled.

## Validation

- Documentation-only change; no runtime/build/workflow/dependency changes or new web behavior is claimed. Current Go APIs and local failure fixtures were inspected.
- `python3 bin/e6-qualification.0Stkvu/yoinkdl-doccheck.py` passed local links/anchors, structured examples, shell syntax, fences, and whitespace, including the new contract. Staged snapshots are checked separately before each commit; `git diff --check` and staged whitespace checks pass.
- Twelve existing focused baseline tests passed in all three packages; exact command:

```sh
go test ./internal/daemon ./internal/queue ./internal/transfer -run '^(TestDaemonClientNeverStartsOrCreatesState|TestDaemonClientLostReplyIsUncertainOnlyForMutations|TestDaemonClientRejectsUnsafeOrMalformedPeer|TestDaemonStateMismatchAndMalformedFramesDoNotMutate|TestProtocolReplyShapesAndPrivacy|TestDaemonDisconnectedAddAndPipelining|TestQueueDisconnectedAddSurvivesWithoutAliasingCaller|TestRecoveryPreservesUnknownOutcomes|TestNewJobNormalizesWithoutFileWork|TestNormalizeRequest|TestNormalizeOwnsExplicitOptionsWithoutFileWork|TestNormalizeInvalidRequestsMatchDownload)$' -count=1
```

- Current Vite and Mozilla authentication/CORS/fetch-metadata documentation informed the contract; primary references are linked there. Existing baseline tests and external documentation do not qualify the new web adapter.
- `make check` and frontend/browser tests were not run for this specification-only work. New W1 acceptance remains pending implementation; native daemon/storage qualification remains Linux/amd64.

## Next steps

1. Implement [W1.1a](docs/local-web.md#delivery-order): pure metadata model/strict codec with stable IDs, Default profile, URL-free immutable submission bindings, revision/reference checks, decimal-string integers, and bounds. Add focused tests, run `make check`, and update STATUS before the focused local commit. Keep filesystem/HTTP/frontend work for later commits.
2. Continue W1.1b ownership/loading, then W1.1c atomic persistence and fault recovery as separate commits; preserve the model contract and unrelated files.
3. Follow W1.2a onward for bounded/authenticated HTTP, safe read views, explicit CLI/assets, and the frontend before enabling profile/admission/control workflows. Keep unfinished features disabled and refresh this handoff after each implementation.
4. Follow W2–W4 after their prerequisites; retain Firefox/hosted tracks L1/L2 as later work.

## Open decisions and blockers

No product decision blocks W1.1a. Choose frontend/tool pins during scaffolding; optional distribution packaging stays deferred. General TOML/cross-platform config discovery, W2+ metadata migrations/library retention, richer telemetry/profile budgets, browser credentials, scheduling, Firefox bridging, and hosted authentication remain separate later contracts.
