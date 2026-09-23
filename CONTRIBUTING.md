# Contributing to goodeiv

Thanks for your interest! This guide covers the essentials. The full set of rules lives in
[AGENTS.md](AGENTS.md) and applies equally to humans and coding agents.

## Before you start

- For anything beyond a small fix, **open an issue first** to discuss the approach.
- Check [docs/roadmap.md](docs/roadmap.md) and [docs/adr](docs/adr/README.md) — the change may
  already be planned or a related decision may exist.

## Setup

```bash
git clone https://github.com/ynxdeiv/goodeiv
cd goodeiv
make setup
make check
```

You need Go 1.26+ and [golangci-lint](https://golangci-lint.run) v2.

## Rules of thumb

- **Small contracts.** Interfaces only when there are real implementations or a testing need.
- **Minimal public API.** If it does not need to be exported, put it in `internal/`.
- **Comments:** short doc comments on exported API only. No comments inside functions, no `TODO`s.
- **Context everywhere.** Every I/O operation takes a `context.Context` and honors cancellation.
- **Errors** use the project taxonomy and wrap causes with `%w`.
- **Tests** describe behavior, include failure cases and pass with `-race`.
- **No new dependencies** in the core without discussion.

## Pull requests

1. Branch from `main`: `feat/<scope>`, `fix/<scope>`, `docs/<scope>` or `chore/<scope>`.
2. Use [Conventional Commits](https://www.conventionalcommits.org) — the `commit-msg` hook checks it.
3. Run `make check` and make sure it is green.
4. Describe *what* and *why* in the PR; link the issue.
5. Significant design decisions get an ADR (`docs/adr/`, see `docs/adr/0000-template.md`).

## Reporting security issues

Please do **not** open a public issue for vulnerabilities. Use
[GitHub private vulnerability reporting](https://github.com/ynxdeiv/goodeiv/security/advisories/new).

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
