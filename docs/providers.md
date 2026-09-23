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

| Adapter | Status |
|---|---|
| `providertest` | ✅ available |
| OpenAI | ⏳ next (phase 03) |
| Anthropic, Gemini and others | 🗓 planned |
