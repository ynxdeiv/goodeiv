---
name: adr
description: Records an architecture decision in docs/adr using the repository template. Use when a decision about a public contract, package boundary, external dependency or data model needs to be justified for the future.
---

# Record an ADR

Input: short decision title.

1. Find the next number: highest `NNNN` in `docs/adr/` + 1, zero-padded to 4 digits.
2. Copy `docs/adr/0000-template.md` to `docs/adr/NNNN-<kebab-case-title>.md`.
3. Fill in: context (the problem, not the solution), decision, alternatives considered and why they were rejected, consequences (good and bad).
4. Initial status `Proposed`; becomes `Accepted` once the maintainer approves.
5. If it replaces an earlier ADR, mark the old one `Superseded by NNNN`.
6. Add a row to the index in `docs/adr/README.md`.

Keep it short. An ADR is not usage documentation — that belongs in `docs/`.
