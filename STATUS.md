# Project status

Updated: October 20, 2026 (America/Sao_Paulo).

## Current state

W1 now defines durable profile binding before Add, CLI-created job handling, and explicit unknown-outcome/cleanup behavior.

No web command, HTTP endpoint, app store, frontend, or frontend dependency is implemented. Existing standalone/Linux daemon behavior is unchanged. Work stays local/no-push; release preparation is outside this build queue.

## Handoff

- [W1 local web](docs/local-web.md) owns the specified workflow and acceptance cases. It is a specification, not a working command or runtime qualification.
- The planned web process serves built Vue 3 + TypeScript/Vite assets and a bearer-authenticated IPv4-loopback API. It attaches to explicit daemon startup; web/tab shutdown never stops daemon work. Node/assets remain optional for existing Go builds.
- The planned private `<daemon-state>/web/app.json` and `app.lock` own stable instance identity, profile defaults, and URL-free submission bindings. Default starts with no output directory. Use bounded strict JSON, decimal-string 64-bit values, owned snapshots, atomic saves, and conservative fail-stop; never edit queue/manifest files.
- Persist a URL-free binding before one daemon Add; never replay it. Freeze profile defaults/overrides, keep CLI jobs Unassigned, and retain unresolved IDs after lost replies. Forget/dismiss preserves transfer files. Reuse `transfer.Normalize`, `queue.NewJob`, `daemon.Resolve`/`Call`, safe views, and existing lifecycle rules.
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

1. Finish the concrete safe HTTP routes, polling, bounded responses, and matching acceptance cases before implementation.
2. Keep later W1 implementation changes focused by responsibility, with relevant tests/docs and a refreshed handoff in each commit.
3. Follow W2–W4 after their prerequisites; retain Firefox/hosted tracks L1/L2 as later work.

## Open decisions and blockers

Complete the remaining W1 definition steps above before dependent implementation. General TOML/cross-platform config discovery, W2+ metadata migrations/library retention, richer telemetry/profile budgets, browser credentials, scheduling, Firefox bridging, and hosted authentication remain separate later contracts.
