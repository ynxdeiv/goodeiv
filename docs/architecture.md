# Architecture

## Layers

```
                 consumer application
                         │
        integrations/* ──┤  (separate modules, implement tool.Tool)
                         │
                       agent            declarative definition: model, tools, limits
                         │
                      runtime           loop, retry, fallback, limits, cancellation
            ┌────────────┼─────────────┬──────────────┐
         provider       tool        approval        policy
            │            │          reference        event
            └────────────┴─────┬───────┴──────────────┘
                           primitives   message, usage, fault
```

Lower-level packages never import higher-level ones. Cycles are solved by redrawing the
boundary, never with a catch-all package.

## Modules

| Module | Contents | Allowed dependencies |
|---|---|---|
| `github.com/ynxdeiv/goodeiv` | core + provider adapters | stdlib, `golang.org/x/*`, vendor SDKs only inside `provider/<name>` |
| `github.com/ynxdeiv/goodeiv/integrations/<name>` | tools for one integration | core + that integration's SDK |

See [ADR 0001](adr/0001-module-boundaries.md).

## Primitives

| Package | Contents |
|---|---|
| `fault` | Error taxonomy (`Kind`, `*Error`, `KindOf`, `IsRetryable`, `RetryAfter`) — see [errors.md](errors.md) |
| `message` | `Role`, `Message` and the closed set of parts: `Text`, `Image`, `File`, `ToolCall`, `ToolResult`; `Trust` marker; validation and JSON |
| `usage` | `Count` (known vs. unreported) and `Usage` with aggregation |

`message` depends on `fault`; `usage` and `fault` depend only on the standard library.

## Providers

`provider` holds the contract, `Request`/`Response`, stream events, `Collect` and the `Registry`;
`provider/providertest` holds the scripted fake. Vendor adapters live in `provider/<name>`.
See [providers.md](providers.md).

## Core contracts (detailed per phase)

| Concept | Responsibility |
|---|---|
| Provider | Generate and stream from a normalized request; declare real capabilities |
| Tool | Definition (name, description, schema, risk) + execution |
| Registry | Registration without singletons, duplicate detection, filtering |
| Runtime | Bounded LLM ↔ tools loop, cancellation, retry, fallback |
| ExecutionContext | Trusted identity (organization, user, conversation, agent) — never model-controlled |
| ReferenceStore | Opaque IDs for external resources, with owner, TTL and consumption state |
| ApprovalStore | Pending actions persisted before execution (HITL) |
| Policy | `allow` / `deny` / `require_approval` decision before execution |
| Observer | Structured events for run, provider, tool, approval, retry, fallback |

## Invariants

- No unbounded loop.
- Approval always happens before execution, followed by revalidation.
- Credentials never appear in prompts, arguments, results, logs or events.
- External content carries an untrusted marker.
- Every I/O operation honors `context.Context`.
