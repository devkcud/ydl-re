# Agent instructions

These instructions apply throughout this repository.

## Start here

1. Read [STATUS.md](STATUS.md) for the current handoff and ordered next steps.
2. Read [README.md](README.md) and the relevant pages in [docs/](docs/README.md).
3. Check the working tree before editing and preserve unrelated local changes.
4. Follow the user's current task. For a continuation request, resume the first unfinished next step in `STATUS.md`.

Use the [roadmap](docs/roadmap.md) for milestone order and [open decisions](docs/decisions.md) for unresolved contracts. Do not treat proposed commands, schemas, or packages as implemented.

## Code style

- Keep explanations short.
- Use short, clear variable names.
- Keep code visually spaced: use standard spaces around operators and after separators, and blank lines between logical blocks. Avoid dense, tightly packed statements while following the language formatter.
- Do not prefix private fields or methods with `_` unless the project does.
- Always declare visibility with `public`, `protected`, or `private` where the language supports it, including when `public` is implicit.
- Apply visibility deliberately to classes, methods, fields, functions, and properties.
- Use `final` or `const` for values that will not change where the language supports them.
- Use `static` for members that belong to the class rather than an instance.
- Do not add comments unless the logic is extremely complex or cannot be made obvious enough in code.

For Go, use exported-name capitalization to express visibility, `const` for valid constants, and package-level functions or values where appropriate. Go has no `public`, `protected`, `private`, `final`, or `static` keywords. Format Go source with `gofmt`.

## Implementation workflow

- Work in the roadmap's dependency order, starting with a reliable single stream.
- Keep changes focused on the requested behavior and reuse existing responsibilities.
- Prefer the standard library. Add packages or dependencies only for a concrete need.
- Preserve the integrity rules: validate responses, never mix resource versions, checkpoint conservatively, prevent simultaneous writers, and publish only verified files.
- Do not silently overwrite completed files or destroy incompatible partial state.
- Update the relevant documentation when behavior or a previously open decision changes.

The standalone CLI comes first. Daemons, queues, TUIs, extra protocols, and workflow engines must not become prerequisites for v0.1.

## Commits

- Always split changes into small, focused commits by responsibility and dependency order. Keep the implementation, tests, and documentation for each behavior together.
- Stage explicit paths or hunks, inspect the staged and remaining diffs, and validate each commit's changes before committing.
- Never push commits, branches, or tags to any remote.

## Validation

Run checks appropriate to the change. For implementation work, test the changed behavior and relevant failure paths, then run the applicable broader checks. Use deterministic fixtures and local HTTP servers.

Once the Go module and source exist, the baseline commands are:

```sh
go build ./...
go test ./...
go vet ./...
go test -race ./...
```

Use a supported environment for race checks. Record failures, skipped checks, and environment limitations accurately. These commands cannot validate the current documentation-only checkout.

For documentation changes, check local links and anchors, examples, and whitespace, including new untracked files.

## Required handoff after every implementation

Update the root [STATUS.md](STATUS.md) after every implementation step, including fixes, refactors, and partially completed work. Do this before ending the work session or reporting completion; do not wait until the whole milestone is finished.

Keep these sections current:

- **Current state:** what is implemented, the active milestone, and what remains unimplemented.
- **Handoff:** what changed, relevant files, decisions, unfinished work, and details needed to continue safely.
- **Validation:** exact commands or checks, outcomes, and anything not run with its reason.
- **Next steps:** concrete tasks in dependency order, with the first unfinished task immediately actionable.
- **Open decisions and blockers:** unresolved choices or failures that affect the next work, with links to supporting context.

Refresh the handoff after documentation or workflow changes that affect how implementation continues. Keep `STATUS.md` a concise current snapshot: replace stale guidance and remove completed next steps instead of appending an unbounded activity log.

Leave enough context for another session to continue without reconstructing the conversation. Never mark a feature or milestone complete without the required implementation and validation evidence.
