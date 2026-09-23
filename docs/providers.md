# Providers

A provider turns a vendor-neutral `provider.Request` into a vendor call and back. See
[ADR 0005](adr/0005-provider-contract.md) for the reasoning behind the contract.

```go
type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) iter.Seq2[Event, error]
	Capabilities(model string) Capabilities
}
```

## Calling a model

```go
resp, err := p.Generate(ctx, provider.Request{
	Model:    "model-name",
	Messages: []message.Message{message.System("Be brief."), message.User("Hi!")},
})
fmt.Println(resp.Message.Text(), resp.FinishReason, resp.Usage)
```

## Streaming

```go
for ev, err := range p.Stream(ctx, req) {
	if err != nil {
		return err
	}
	if ev.Type == provider.EventTextDelta {
		fmt.Print(ev.Delta)
	}
}
```

Breaking out of the loop or canceling `ctx` stops the stream and releases its resources.

| Event | Fields |
|---|---|
| `text_delta` | `Delta` |
| `reasoning_delta` | `Delta` |
| `tool_call_started` | `ToolCall.ID`, `ToolCall.Name` |
| `tool_call_delta` | `ToolCall.ID`, argument fragment in `Delta` |
| `tool_call_completed` | full `ToolCall` |
| `usage` | usage snapshot — known fields replace earlier values |
| `completed` | `FinishReason`, `Model`, `RequestID` — always last |

## Registry

```go
providers := provider.NewRegistry()
if err := providers.Register("openai", openaiProvider); err != nil { ... }
p, err := providers.Get("openai")
```

No global registry exists; pass it explicitly.

## Validation and capabilities

- `req.Validate()` checks the request before any tokens are spent: model set, system messages
  first, valid parts, every tool result answering an earlier tool call, unique tool names,
  JSON-object schemas and a consistent tool choice.
- `p.Capabilities(model).Supports(req)` rejects tools, images or files the model cannot handle.

## Testing with the fake provider

```go
fake := providertest.New(
	providertest.CallTools(message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{"city":"Lisbon"}`)}),
	providertest.Reply("It is sunny in Lisbon."),
)
```

Turns: `Reply`, `CallTools`, `Fail`, `Interrupt` (partial output, then an error), plus
`.WithUsage(u)`. `fake.Requests()` returns what the model was sent.

## Writing an adapter

1. Keep every vendor type unexported inside `provider/<name>`.
2. Implement `Stream`; implement `Generate` as `provider.Collect(p.Stream(ctx, req))` unless the
   vendor's blocking API is materially better.
3. Validate with `req.Validate()` and `Capabilities(req.Model).Supports(req)` before calling out.
4. Map vendor failures to `fault` kinds, attaching `RetryAfter` when the vendor sends it.
5. Emit `tool_call_completed` for every started call and finish with `completed`.
6. Declare only capabilities the model really has.

## Adapters

| Adapter | Package | Status |
|---|---|---|
| Fake (tests) | `provider/providertest` | ✅ available |
| DeepSeek | `provider/deepseek` | ✅ available |
| OpenAI, Anthropic, Gemini and others | — | 🗓 planned |

### DeepSeek

```go
p, err := deepseek.New(deepseek.Config{APIKey: os.Getenv("DEEPSEEK_API_KEY")})
resp, err := p.Generate(ctx, provider.Request{Model: model, Messages: msgs})
```

- Models (verified live on 2026-09-23): `deepseek-flash` and `deepseek-v4-pro`.
- Text-only thinking models: `Capabilities` reports tools, parallel tool calls, reasoning and
  prompt caching — no vision, files, schema-constrained output or forced tool choice.
- Thinking mode rejects `tool_choice` `required` or a named tool with HTTP 400, even though the
  API reference lists them; `auto` and `none` work. goodeiv rejects forced choices before sending.
- Reasoning arrives as `message.Reasoning` parts. When a request has tools, DeepSeek requires the
  reasoning of earlier assistant turns to be sent back; the adapter does it as long as you keep the
  assistant messages you received in the conversation.
- Tool names such as `gmail.search` are sent as `gmail_search` and mapped back transparently. Two
  tools that map to the same wire name are rejected before any request is made.
- HTTP 402 (no balance) is `quota_exhausted`, 429 is `rate_limited` with `Retry-After`, 503 is
  `provider_unavailable`, and `insufficient_system_resource` mid-generation is `provider_unavailable`.

Adapters built on the Chat Completions format share `internal/chatcompletions`, which owns request
mapping, SSE parsing, tool-name mapping and error classification.

## Live tests

Tests against real APIs are behind the `live` build tag and never run in CI. Put credentials in an
untracked `.env` file:

```bash
DEEPSEEK_API_KEY=...
DEEPSEEK_MODEL=deepseek-flash   # run once without it to list the available models
```

```bash
make live
```
