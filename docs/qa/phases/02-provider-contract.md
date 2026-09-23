# Phase `02-provider-contract` — Provider contract

## Goal

Define how the runtime talks to any LLM vendor, and make it testable without a network.

## API added or changed

| Package | Public API |
|---|---|
| `provider` | `Provider` (`Generate`, `Stream` as `iter.Seq2[Event, error]`, `Capabilities`), `Capabilities` + `Supports`, `Request` + `Validate`, `ToolSpec`, `ToolChoice`/`ToolChoiceMode` (zero value = auto), `Response`, `FinishReason` + `Valid`, `Event`/`EventType`, `Collect`, `Registry` (`NewRegistry`, `Register`, `Get`, `Names`) |
| `provider/providertest` | `New`, `Turn` (`Reply`, `CallTools`, `Fail`, `Interrupt`, `WithUsage`), `Provider.WithCapabilities`, `Requests`, `Remaining` |

Design notes ([ADR 0005](../../adr/0005-provider-contract.md)):

- Streams are `iter.Seq2`: breaking the loop is the only cleanup a consumer needs.
- `Collect` turns any stream into a `Response`, so adapters implement streaming once.
- A response with tool calls always reports `tool_calls`, because some vendors report `stop` in that case.
- Usage events are snapshots merged field by field, which fits vendors that send input tokens first and output tokens at the end.
- `Request.Validate` rejects orphan tool results and duplicate tool call IDs — a common cause of vendor 400s — before any tokens are spent.

## Files

- `provider/provider.go`, `provider/request.go`, `provider/stream.go`, `provider/registry.go`
- `provider/request_test.go`, `provider/stream_test.go`, `provider/registry_test.go`
- `provider/providertest/providertest.go`, `provider/providertest/providertest_test.go`
- Docs: `docs/providers.md`, `docs/adr/0005-provider-contract.md`, `docs/adr/README.md`, `docs/architecture.md`, `docs/roadmap.md`, `README.md`

## Tests added

All external (`package xxx_test`).

- Request validation: 24 cases (model, message ordering, orphan results, duplicate call IDs, tool names/schemas, every tool-choice rule, token and temperature bounds).
- Capabilities: tools, images and files against supported/unsupported models.
- Collect: text assembly, part order across text and tool calls, forced `tool_calls` finish reason, usage snapshot merging, and failures (stream error passthrough, missing completion, unfinished tool call, events after completion, unknown event type).
- Registry: register/get, empty name, nil provider, duplicates, unknown name, sorted names, concurrent use under `-race`.
- providertest: turn ordering and request recording, exhausted script, scripted failures, partial output then failure, invalid requests not consuming turns, canceled context, consumer break, usage, capabilities, runnable `Example`.

## Commands run

```text
$ make check
go vet ./...
go run ./internal/tools/nocomments .
golangci-lint run
0 issues.
go test -race ./...
ok  	github.com/ynxdeiv/goodeiv/fault
ok  	github.com/ynxdeiv/goodeiv/internal/tools/nocomments
ok  	github.com/ynxdeiv/goodeiv/message
ok  	github.com/ynxdeiv/goodeiv/provider
ok  	github.com/ynxdeiv/goodeiv/provider/providertest
ok  	github.com/ynxdeiv/goodeiv/usage

$ go test -count=1 -cover ./provider/...
ok  	github.com/ynxdeiv/goodeiv/provider	coverage: 100.0% of statements
ok  	github.com/ynxdeiv/goodeiv/provider/providertest	coverage: 100.0% of statements
```

## Result

Provider contract, validation, stream assembly, registry and a scripted fake provider, all green.
Standard library only.

## Remaining risks

- Tool names allow `.`, which some vendors reject; adapters must map names reversibly (phase 03).
- `Requests()` returns shallow copies; a test mutating recorded messages mutates the record.
- Reasoning deltas are not kept in the assembled message; revisit if a consumer needs them.

## Next step

Phase `03-provider-openai`: first real adapter, built on `httptest` fixtures.
