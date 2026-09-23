# 0002 — Comments only as doc comments on exported API

- Status: Accepted
- Date: 2026-09-22

## Context

Implementation comments drift away from the code and tend to compensate for poor names and
boundaries. On the other hand, a Go library is discovered and learned through pkg.go.dev and
editor hovers, which are built entirely from doc comments on exported identifiers.

## Decision

- Doc comments are allowed — and expected — on the package clause and on exported types,
  functions, methods of exported types, constants, variables, struct fields and interface methods.
  They are short, start with the identifier name and describe the contract.
- Everything else is forbidden: comments inside function bodies, trailing comments, comments on
  unexported code, and task markers (`TODO`, `FIXME`, `XXX`, `HACK`) anywhere.
- Directives (`//go:`, `//nolint:`, `//line`, `//export`) and generated files are exempt.
- Longer explanations, invariants and decisions live in `docs/` and `docs/adr/`.
- Enforced by `internal/tools/nocomments` in `make check`, the pre-commit hook and CI.

## Alternatives considered

- **No comments at all**: the public API would render empty on pkg.go.dev, hurting discoverability and adoption.
- **Free-form comments**: implementation comments rot and hide naming problems.
- **Convention only**: does not hold up with multiple coding agents generating code.

## Consequences

- pkg.go.dev and IDE hovers document the public API.
- Implementation readability depends on naming and structure — reviews focus on them.
- Runnable examples (`Example*` in `_test.go`) remain the preferred way to document usage.
