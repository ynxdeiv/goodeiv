<h1 align="center">goodeiv</h1>

<p align="center">
  <b>A secure, bounded agent runtime for Go.</b><br>
  LLM providers · tool calling · agent loop · human-in-the-loop approvals · references · policies · observability
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/ynxdeiv/goodeiv"><img src="https://pkg.go.dev/badge/github.com/ynxdeiv/goodeiv.svg" alt="Go Reference"></a>
  <a href="https://github.com/ynxdeiv/goodeiv/actions/workflows/ci.yml"><img src="https://github.com/ynxdeiv/goodeiv/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/ynxdeiv/goodeiv"><img src="https://goreportcard.com/badge/github.com/ynxdeiv/goodeiv" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"></a>
</p>

---

> [!WARNING]
> **Pre-alpha.** goodeiv is under active development and has no tagged release yet.
> The API shown below is the design target and may change. Follow the [roadmap](docs/roadmap.md).

## Table of contents

- [Why goodeiv](#why-goodeiv)
- [Features](#features)
- [How it works](#how-it-works)
- [Planned usage](#planned-usage)
- [Core concepts](#core-concepts)
- [Architecture](#architecture)
- [Project status](#project-status)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Why goodeiv

Calling an LLM is easy. Running an **agent in production** is not:

- The model decides to call tools — but **who is allowed** to do what?
- A tool sends an email or deletes a file — **who approved it**, and was it approved **before** it ran?
- The model passes IDs between turns — what stops it from **inventing or swapping** them?
- A provider returns 429 mid-conversation — does the loop **retry safely** without sending the email twice?
- The loop keeps calling tools — what **stops it**?

Most libraries leave these questions to you. goodeiv makes them part of the runtime, so every
product built on top of it gets the same guarantees by default.

**goodeiv is not a framework.** It is a set of small, composable Go packages with explicit
dependencies, no global state, no reflection-based magic and no vendor lock-in.

## Features

| | |
|---|---|
| 🔌 **Provider-agnostic** | One normalized request/response model. Adapters for OpenAI, Anthropic, Gemini and more. Vendor types never leak into your code. |
| 🧰 **Tools** | Small tool contract, concurrency-safe registry, validation and a staged execution pipeline. |
| 🔁 **Bounded agent loop** | Hard limits on steps, tool calls, tokens and duration. An infinite loop is impossible by construction. |
| ✋ **Human-in-the-loop** | Risky actions are persisted and approved **before** execution, then revalidated. Rejection means the action never runs. |
| 🔐 **Trusted identity** | User, organization and credentials come from the execution context — never from model-generated arguments. |
| 🔗 **References** | The model sees opaque IDs; the real resource, owner, TTL and consumption state live in a store you control. |
| 🛡️ **Policies** | `allow` / `deny` / `require_approval` decisions before any tool runs. |
| ♻️ **Retry & fallback** | Retries only recoverable errors, honors `Retry-After`, backoff with jitter, and falls back across models/providers. |
| 🧾 **Idempotency** | Mutating actions carry idempotency keys, so retries and double clicks never duplicate side effects. |
| 📡 **Streaming** | Normalized stream events (text, reasoning, tool calls, usage) with proper cancellation and cleanup. |
| 🧱 **Structured output** | Typed, validated generation that fails loudly instead of silently. |
| 📊 **Observability** | Structured events, correlated trace IDs and normalized usage — plug in OpenTelemetry, logs or anything else. |
| ❗ **Normalized errors** | One error taxonomy with `errors.Is` / `errors.As`, raw causes preserved. |

## How it works

```
 user input
     │
     ▼
┌──────────────────────────── runtime.Run ───────────────────────────┐
│                                                                    │
│   provider ──► response ──► tool calls? ── no ──► final answer     │
│      ▲                          │                                  │
│      │                         yes                                 │
│      │                          ▼                                  │
│      │     validate ─► policy ─► risk ─► approval? ─► execute      │
│      │                                      │            │         │
│      │                              pause & persist      │         │
│      │                              (resume later)       ▼         │
│      └──────────────── normalized tool results ◄── references      │
│                                                                    │
│   limits: steps · tool calls · tokens · duration · context cancel  │
└────────────────────────────────────────────────────────────────────┘
     │
     ▼
 events · traces · usage
```

## Planned usage

> The snippets below illustrate the target API. They will be validated against real code as each
> [roadmap](docs/roadmap.md) phase lands.

### Install

```bash
go get github.com/ynxdeiv/goodeiv
```

### Register a provider

```go
providers := provider.NewRegistry()
providers.Register("openai", openai.New(openai.Config{APIKey: os.Getenv("OPENAI_API_KEY")}))
```

### Define a tool

```go
type weather struct{}

func (weather) Definition() tool.Definition {
	return tool.Definition{
		Name:        "weather.current",
		Description: "Current weather for a city.",
		Schema:      tool.MustSchema[WeatherInput](),
		Risk:        tool.RiskRead,
	}
}

func (weather) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var in WeatherInput
	if err := call.Decode(&in); err != nil {
		return tool.Result{}, err
	}
	return tool.Text(fmt.Sprintf("22°C and sunny in %s", in.City)), nil
}
```

### Run an agent

```go
tools := tool.NewRegistry()
tools.Register(weather{})

rt := runtime.New(runtime.Config{
	Providers: providers,
	Tools:     tools,
	Approvals: approvalStore,
	Observer:  observer,
})

result, err := rt.Run(ctx, runtime.RunRequest{
	Agent: agent.Agent{
		Name:  "assistant",
		Model: model.Model{Provider: "openai", Name: "gpt-5"},
		Tools: []string{"weather.current"},
		Limits: agent.Limits{MaxSteps: 8, MaxToolCalls: 16},
	},
	Input:    "Do I need an umbrella in Lisbon today?",
	Identity: runtime.Identity{OrganizationID: orgID, UserID: userID},
})
```

### Handle approvals

```go
if errors.Is(err, runtime.ErrApprovalRequired) {
	pending := result.PendingApproval
	// show it to a human, then:
	result, err = rt.Resume(ctx, pending.ID, approval.Approve)
}
```

## Core concepts

| Concept | What it is |
|---|---|
| **Provider** | Adapter that turns a normalized request into a vendor call and back, declaring its real capabilities. |
| **Tool** | A definition (name, description, input schema, risk) plus an `Execute` function. |
| **Registry** | Where tools and providers are registered — no singletons, duplicates rejected. |
| **Runtime** | Runs the bounded loop between the model and tools. |
| **Agent** | Declarative configuration: model, allowed tools, limits. |
| **Identity / ExecutionContext** | Trusted caller identity, set by your application — never by the model. |
| **Reference** | Opaque handle to an external resource, scoped to an owner and a TTL. |
| **Policy** | Decides whether a tool call is allowed, denied or needs approval. |
| **Approval** | A persisted pending action awaiting a human decision. |
| **Observer** | Receives structured events for everything the runtime does. |

Storage (`ReferenceStore`, `ApprovalStore`) is defined as small interfaces. goodeiv ships
in-memory implementations for tests and development; bring your own Postgres, Redis or anything else
for production.

## Architecture

```
primitives (message, model, usage, errors)
   ↑
provider/*   tool   reference   approval   policy   event
   ↑
runtime (agent loop, retry, fallback, limits)
   ↑
agent / composition API
   ↑
integrations/* (separate Go modules)   ←   your application
```

- The **core** module depends only on the standard library and minimal, justified packages.
- **Vendor SDKs** are confined to `provider/<name>`.
- **Integrations** (Google, Slack, CRMs…) live in `integrations/<name>` as separate Go modules, so
  importing the core never pulls in their dependencies.

Read more in [docs/architecture.md](docs/architecture.md) and the [ADRs](docs/adr/README.md).

## Project status

| Area | Status |
|---|---|
| Repository foundation, tooling, CI | ✅ done |
| Core types and error taxonomy | ✅ done |
| Provider contract + OpenAI adapter | ⏳ next |
| Tools, registry and agent loop | 🗓 planned |
| Retry and fallback | 🗓 planned |
| References, HITL and policies | 🗓 planned |
| Observability and structured output | 🗓 planned |
| Anthropic, Gemini and more providers | 🗓 planned |
| First integration module (Google) | 🗓 planned |

Full breakdown in [docs/roadmap.md](docs/roadmap.md).

## Development

Requirements: Go 1.26+, `make`, and [golangci-lint](https://golangci-lint.run) v2.

```bash
git clone https://github.com/ynxdeiv/goodeiv
cd goodeiv
make setup    # enable git hooks
make check    # fmt, vet, comment policy, lint, tests with -race
```

| Command | What it does |
|---|---|
| `make check` | Every gate — run before any PR |
| `make test` / `make race` | Tests, with and without the race detector |
| `make lint` | golangci-lint |
| `make nocomments` | Comment policy: doc comments only on exported API |
| `make fmt` | gofmt |

### Built with coding agents

This repository is set up to be worked on by humans and AI coding agents alike (Codex, Claude
Code, Cursor, opencode…). [AGENTS.md](AGENTS.md) is the single source of rules; specialized roles
and workflows live in [`.agents/`](.agents). All rules are enforced by `make`, git hooks and CI —
not by trust.

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and
[AGENTS.md](AGENTS.md) first — the short version:

- Open an issue before large changes.
- Keep contracts small and public API minimal.
- Doc comments only on exported API; no comments inside functions.
- `make check` must be green.
- Conventional Commits (`feat(tool): add registry`).

## License

[Apache License 2.0](LICENSE) © goodeiv contributors.
