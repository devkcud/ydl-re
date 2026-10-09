# Project status

Updated: October 9, 2026.

## Current state

- Design and documentation are in place, including 16 guides under [docs/](docs/README.md).
- Root [README.md](README.md) introduces the project; [AGENTS.md](AGENTS.md) defines the implementation and handoff workflow.
- Implementation has not started. There is no Go module, source tree, executable, test suite, CI, or release tooling.
- The next milestone is [A: reliable single stream](docs/roadmap.md#milestone-a--reliable-single-stream). All implementation milestones remain pending.

## Handoff

The documentation and contributor workflow are in place. `AGENTS.md` now requires splitting changes into focused commits by responsibility and dependency order, validating each commit, and never pushing to any remote.

Every implementation step must leave an updated handoff and ordered next steps here, including partial work and failed validation. Replace outdated status rather than accumulating a session log.

The existing [engineering design](yoinkdl-design.md) remains unchanged. CLI examples, schemas, and package layouts are still proposals; do not infer working behavior from them. No application code was added in this update.

## Validation

- Previous documentation audit passed: 19 authored Markdown files, covering 184 local links and anchors, code fences, JSON/TOML examples, required status sections, and the unchanged original design checksum.
- Previous example and whitespace checks passed: `zsh -n` for 18 shell examples and `git diff --no-index --check /dev/null <file>` for all 19 authored Markdown files.
- Passed: local link/anchor and whitespace checks for the commit-workflow update to `AGENTS.md` and `STATUS.md`.
- Not run: Go build, tests, vet, and race checks. There is no Go module or implementation to validate.

## Next steps

1. Settle the initial Milestone A choices: Go version, module path, minimal CLI parser, destination policy, and a platform-qualified publication strategy. Record the choices in the owning docs and [decision register](docs/decisions.md).
2. Scaffold only the packages needed for `download` and `version`, then implement a foreground HTTP(S) stream into a separate partial file with progress, cancellation, response/length checks, and safe publication.
3. Add deterministic local-server tests for exact output, bad statuses, incomplete bodies, unknown lengths, interruption, destination collisions, and publication failures.
4. Run focused tests and the applicable build, test, vet, and race checks. Record actual results and platform limits here; update docs as proposed behavior becomes implemented.
5. Begin [Milestone B](docs/roadmap.md#milestone-b--persistence-and-resume) only after Milestone A has its required evidence. Add persistence, resource identity, exclusive ownership, safe resume, and `inspect` in that order as their dependencies permit.

Update this handoff after each implementation step within these tasks, not just after the entire list or milestone.

## Open decisions and blockers

The next work requires the initial build and file-publication choices above. The [open decisions](docs/decisions.md) cover the remaining command, storage, HTTP, and release contracts; resolve them when their implementation depends on them.

No external blocker was found for the documentation/workflow update. License and distribution choices remain open before release. Platform support and atomic publication have not been validated.
