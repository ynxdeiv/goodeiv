---
name: phase
description: Runs one phase from docs/roadmap.md end to end with an evidence gate — branch, tests first, implementation, make check, record in docs/qa/phases and final review. Use when asked to implement or advance a roadmap phase.
---

# Run a phase

Input: phase id (e.g. `01-core-types`).

1. Read the phase in `docs/roadmap.md`, `docs/architecture.md` and related ADRs. If `.local/CONTEXT.md` exists, read it too.
2. Create branch `feat/<phase>` from an up-to-date `main`.
3. Read the matching role in `.agents/roles/` and follow its invariants.
4. Write behavior tests first (including one external `package xxx_test` test). Confirm they fail for the right reason.
5. Implement the minimum to make them pass. Doc comments only on exported API. No unnecessary exports.
6. Run `make check`. Fix until green — do not skip steps or disable linters.
7. Review with the `go-qa-reviewer` role. Fix P0/P1 findings.
8. Record `docs/qa/phases/<phase>.md` from `docs/qa/phases/TEMPLATE.md`, with real command output.
9. If a relevant architectural decision was made, use the `adr` skill.
10. Mark the phase as done in `docs/roadmap.md`.

Do not commit or push without an explicit request from the maintainer.
