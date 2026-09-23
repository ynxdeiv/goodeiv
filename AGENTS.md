# AGENTS.md — goodeiv

Instructions for any coding agent (Codex, Claude Code, Cursor, opencode, Cline, Aider,
Gemini CLI…) and for humans. This file is the single source of truth; tool-specific files
(`CLAUDE.md` etc.) only point here.

`goodeiv` is a Go **agent runtime** SDK: LLM providers, tools, agent loop, human-in-the-loop,
references, policies and observability — a reusable, product-agnostic foundation.
Concrete integrations (Google, Slack, CRMs…) **implement** the runtime contracts; the runtime
never knows about them.

---

## 1. Sources of truth

| What | Where |
|---|---|
| Architecture overview | `docs/architecture.md` |
| Architecture decisions | `docs/adr/` |
| Phased roadmap | `docs/roadmap.md` |
| Per-phase evidence | `docs/qa/phases/<phase>.md` |
| Specialized roles | `.agents/roles/*.md` |
| Reusable workflows | `.agents/skills/*/SKILL.md` |
| Maintainer's private context | `.local/CONTEXT.md` — **read it if present**; not versioned |

Nothing from `.local/` or from `*.local.*` files may appear in versioned files, commit messages,
branch names or PR descriptions.

---

## 2. Hard rule: comments only as doc comments on exported API

- **Allowed:** short doc comments on the package clause and on exported types, functions,
  methods (of exported types), constants, variables, struct fields and interface methods.
  Follow Go style: start with the identifier name, one or two sentences, describe *what* and
  the contract — not *how*.
- **Allowed:** directives (`//go:build`, `//go:generate`, `//go:embed`, `//nolint:<linter>`, `//line`, `//export`).
- **Forbidden:** comments inside function bodies, trailing comments, comments on unexported
  code, and `TODO`/`FIXME`/`XXX`/`HACK` anywhere. If code needs an explanation, rename or split it.
- Generated files (`Code generated ... DO NOT EDIT.`) are skipped.
- Longer conceptual documentation lives in `docs/`.
- Enforced by `make nocomments`, the pre-commit hook (`.githooks/`) and CI. If it fails, fix the
  code — do not work around it.

```go
// Registry stores tools by name and rejects duplicates.
type Registry struct{ ... }

// Register adds t to the registry. It returns ErrDuplicateTool if the name is taken.
func (r *Registry) Register(t Tool) error {
	if r.has(t.Name()) {
		return ErrDuplicateTool
	}
	...
}
```

---

## 3. Target architecture (summary — details in `docs/architecture.md`)

Dependency direction (bottom-up; never the reverse):

```
primitives (message, model, usage, errors…)
   ↑
provider/*   tool   reference   approval   policy   event
   ↑
runtime (agent loop, retry, fallback, limits)
   ↑
agent / composition API
   ↑
integrations/* (separate Go modules)   ←   consumer applications
```

- **Core = root module.** Stdlib plus minimal, justified dependencies only.
- **Provider adapters** encapsulate the vendor SDK/HTTP API; vendor types never leak.
- **`integrations/<name>/`** has its own `go.mod` (ADR 0001) — the core cannot import integration SDKs.
- Implementation details live in `internal/`. Everything outside `internal/` is public API: keep it small.
- Do not create packages beyond what `docs/architecture.md` and the roadmap describe without recording the decision.

### Forbidden

`utils`, `helpers`, `common`, `shared`, `misc`, `base`, `manager`, `service` as package names;
singletons/global state; service locators; DI containers; `context.Background()` inside a
request-scoped flow; `panic` for normal control flow; `map[string]any` as a domain model; identity
(`user_id`, `org_id`, credentials) taken from tool arguments; logging tokens/secrets; LangChain,
LlamaIndex, Nango or agent frameworks.

---

## 4. Go conventions

- Constructor injection; required dependencies are parameters, not options.
- `context.Context` as the first parameter of every external or long-running operation.
- Errors: normalized taxonomy, wrapping with `%w`, `errors.Is/As`. Never expose raw provider errors.
- Typed enums for closed sets (`Role`, `FinishReason`, `Risk`, `Decision`…).
- Pointers only when absence differs from the zero value.
- Generics only when they remove real duplication and improve type safety.
- Every goroutine has a clear lifecycle (`errgroup`, cancellation, bounded concurrency).
- Configurable timeouts — no magic numbers.
- Behavior-driven tests; fake provider and fake tools to test the loop without network.
- At least one external test (`package xxx_test`) per public package to validate ergonomics.
- Prefer runnable `Example*` functions in `_test.go` files to document usage.

---

## 5. Commands

```bash
make setup       # enable the repository git hooks (run once after cloning)
make check       # fmt-check + vet + nocomments + leakcheck + lint + race (gate before any PR)
make test        # go test ./...
make race        # go test -race ./...
make vet
make lint        # golangci-lint (install: brew install golangci-lint)
make nocomments
make fmt
```

A task is done only when `make check` is green **and** evidence is recorded (§6).
After editing any `.go` file, run `make fmt nocomments` before moving on.

---

## 6. Workflow

1. Each phase is described in `docs/roadmap.md`.
2. Per phase: tests → implementation → `make check` → `docs/qa/phases/<phase>.md` (evidence gate).
3. Relevant decision → new ADR in `docs/adr/NNNN-title.md`.

Detailed workflows: `.agents/skills/phase/SKILL.md` (run a phase) and
`.agents/skills/adr/SKILL.md` (record a decision).

---

## 7. Git

- `main` is protected. Branches: `feat/<scope>`, `fix/<scope>`, `docs/<scope>`, `chore/<scope>`.
- Conventional Commits (`feat(tool): …`, `docs(adr): …`), enforced by the `commit-msg` hook.
- 1 phase = 1 branch = 1 PR.
- Never blind `git add -A`, `git reset --hard` or `push --force` to `main`.
- All versioned content (code, docs, commits, PRs) is written in English.

---

## 8. Specialized roles (`.agents/roles/`)

When working in an area, read the matching role and follow its invariants. Tools with
subagents (Claude Code, opencode…) can run them as separate agents; others use the file
as a checklist.

| Role | When |
|---|---|
| `runtime-architect` | Package design, public API, dependency graph, ADRs |
| `provider-adapter-engineer` | Provider contract and adapters (OpenAI, Anthropic, Gemini…), streaming, retry/fallback |
| `tool-runtime-engineer` | Tool contract, registry, executor, agent loop, HITL, policy, references |
| `integration-engineer` | Modules under `integrations/*` implementing runtime contracts |
| `go-qa-reviewer` | Final review of each phase: tests, race, context, errors, comment policy, boundaries |
