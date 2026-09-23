# Phase `01-core-types` — Core primitives

## Goal

Provide the provider-agnostic primitives every other package builds on: the conversation model,
token usage and the error taxonomy.

## API added or changed

| Package | Public API |
|---|---|
| `fault` | `Kind` (24 kinds, implements `error`), `Kind.Valid`, `Kind.Retryable`, `Error{Kind, Message, RetryAfter, Err}`, `New`, `Wrap`, `(*Error).WithRetryAfter`, `KindOf`, `IsRetryable`, `RetryAfter` |
| `message` | `Role` (+ `RoleSystem/User/Assistant/Tool`, `Valid`), `Message{Role, Parts}`, `New`, `System`, `User`, `Assistant`, `Tool`, `Text()`, `ToolCalls()`, `ToolResults()`, `Validate()`, JSON encoding; `Part` (sealed): `Text`, `Image`, `File`, `ToolCall`, `ToolResult`; `Trust` (`Untrusted` zero value, `Trusted`) |
| `usage` | `Count` (`Known`, `Value`, `IsKnown`, `Add`, JSON null for unknown), `Usage{InputTokens, OutputTokens, CachedInputTokens, CacheWriteTokens, ReasoningTokens}`, `Add`, `TotalTokens` |

Design notes:

- `fault.Error.Error()` never prints the cause, so vendor payloads cannot leak through logs ([ADR 0004](../../adr/0004-error-model.md)).
- `quota_exhausted` is distinct from `rate_limited` because some providers answer HTTP 429 for exhausted billing quota, which must not be retried.
- `ToolResult.Trust` defaults to `Untrusted`, so external content is guarded unless a tool explicitly vouches for it.
- `usage.Count` separates "not reported" from zero; aggregation never invents values.
- The finish reason moved to phase 02 (it belongs to provider responses).

## Files

- `fault/fault.go`, `fault/fault_test.go`
- `message/message.go`, `message/part.go`, `message/validate.go`, `message/json.go`, `message/message_test.go`
- `usage/usage.go`, `usage/usage_test.go`
- `internal/tools/nocomments`: allows `// Output:` blocks inside `Example*` functions of `_test.go` files (needed by `go test`), with tests.
- Docs: `docs/errors.md`, `docs/adr/0004-error-model.md`, `docs/architecture.md`, `docs/roadmap.md`, `README.md`.

## Tests added

All tests are external (`package xxx_test`), including runnable examples.

- `fault`: `errors.Is` by kind, cause preserved but not printed, `errors.As` details, `KindOf` for fault/kind/context/unknown errors, retryability table (including caller cancellation), `RetryAfter`, immutability of `WithRetryAfter`, `Kind.Valid`, `ExampleKindOf`.
- `message`: constructors, text extraction, tool call/result accessors, untrusted default, 25-case validation table (roles × part types, tool call IDs/names/JSON objects, `null` arguments, media sources), JSON round trip of a full conversation, rejection of unknown part types and trust levels, `ExampleNew`.
- `usage`: unknown vs. zero, `Add` truth table, field-wise aggregation, total tokens, JSON omission of unknown fields and round trip, `null` decoding, `ExampleUsage_Add`.

Bug found during review and fixed test-first: tool call arguments equal to JSON `null` were accepted as an object.

## Commands run

```text
$ make check
gofmt -l .                      (clean)
go vet ./...
go run ./internal/tools/nocomments .
golangci-lint run
0 issues.
go test -race ./...
ok  	github.com/ynxdeiv/goodeiv/fault
ok  	github.com/ynxdeiv/goodeiv/internal/tools/nocomments
ok  	github.com/ynxdeiv/goodeiv/message
ok  	github.com/ynxdeiv/goodeiv/usage

$ go test -race -count=1 -cover ./...
ok  	github.com/ynxdeiv/goodeiv/fault	coverage: 100.0% of statements
ok  	github.com/ynxdeiv/goodeiv/internal/tools/nocomments	coverage: 68.1% of statements
ok  	github.com/ynxdeiv/goodeiv/message	coverage: 94.7% of statements
ok  	github.com/ynxdeiv/goodeiv/usage	coverage: 89.5% of statements
```

## Result

Primitives implemented, documented and green under every gate. Only the standard library is used.

## Remaining risks

- `fault.Kind` values are listed twice (constants and the validity set); a new kind must be added to both.
- `message` has no provider mapping yet; the shape may still change when the first adapters land (phase 03), which is acceptable pre-1.0.
- `ToolResult` carries text only; image tool results can be added later as an additive field.

## Next step

Phase `02-provider-contract`: request/response, finish reason, capabilities, provider registry and a scripted fake provider.
