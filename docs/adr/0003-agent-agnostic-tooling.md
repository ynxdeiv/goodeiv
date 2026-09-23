# 0003 — Tool-agnostic coding-agent configuration

- Status: Accepted
- Date: 2026-09-22

## Context

The project is developed with several coding agents (Codex, Claude Code, Cursor, opencode, and
models such as DeepSeek through those tools). Per-tool copies of the rules drift apart.

## Decision

- `AGENTS.md` is the single source of instructions. Tool-specific files (`CLAUDE.md`) only import it.
- Specialized roles live in `.agents/roles/*.md` and workflows in `.agents/skills/*/SKILL.md`, as markdown with minimal frontmatter (`name`, `description`).
- Tool-specific directories (`.claude/agents`, `.claude/skills`) are symlinks to `.agents/`.
- Mandatory rules are enforced outside any agent: `Makefile`, git hooks in `.githooks/` and CI. Tool hooks are a convenience only.

## Alternatives considered

- **Configuration for a single tool**: locks the project to one vendor.
- **Per-tool copies**: diverge on the first edit.

## Consequences

- Supporting a new tool = one file or symlink pointing at `AGENTS.md` and `.agents/`.
- Symlinks require `core.symlinks=true` on Windows.
