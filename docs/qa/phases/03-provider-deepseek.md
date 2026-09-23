# Phase `03-provider-deepseek` — DeepSeek adapter

## Goal

First real provider: DeepSeek, built on a reusable core for the OpenAI-compatible Chat Completions
format so other compatible vendors become cheap to add.

## API added or changed

| Package | Change |
|---|---|
| `message` | New `Reasoning` part (assistant only) and `Message.Reasoning()`. `Text()` ignores reasoning. |
| `provider` | `Collect` keeps reasoning deltas as `message.Reasoning` parts instead of discarding them. New `Capabilities.ForcedToolChoice`, enforced by `Supports`. |
| `provider/deepseek` | `New(Config)`, `Config{APIKey, BaseURL, HTTPClient}`, `DefaultBaseURL`, `DefaultResponseHeaderTimeout`, `Provider` (`Generate`, `Stream`, `Capabilities`). |
| `internal/chatcompletions` | Request mapping, SSE parsing, reversible tool-name mapping, HTTP/stream error classification. Not public. |

Behavior verified against the vendor documentation (thinking mode, chat completion reference,
error codes):

- Reasoning of earlier assistant turns is sent back as `reasoning_content` when tools are present,
  which the vendor requires in thinking mode. Kept as real content, not an empty placeholder.
- Tool names are mapped to the vendor's allowed character set and back; collisions fail before any request.
- Forced tool choices (`required`, named tool) are rejected before sending, because the live API
  answers HTTP 400 "Thinking mode does not support this tool_choice" on both models, contradicting
  the API reference. `auto` and `none` are sent as is.
- Errors: 400 → `invalid_input` (or `context_too_large`), 401 → `unauthenticated`, 402 →
  `quota_exhausted`, 403 → `unauthorized`, 422 → `invalid_input`, 429 → `rate_limited` +
  `Retry-After`, 500 → `temporary_failure`, 503 → `provider_unavailable`, 504 → `timeout`.
  Vendor messages stay in the unwrapped cause, never in `Error()`.
- Stream: keep-alive comments ignored; mid-stream error chunks, malformed chunks and connections
  closed before `[DONE]` are `stream_failed`; `insufficient_system_resource` is `provider_unavailable`.
- No `Client.Timeout`; the default client bounds only the wait for response headers.

## Files

- `message/part.go`, `message/message.go`, `message/validate.go`, `message/json.go`, `message/message_test.go`
- `provider/stream.go`, `provider/stream_test.go`
- `internal/chatcompletions/{client,request,stream,errors,wire}.go`
- `provider/deepseek/deepseek.go`, `provider/deepseek/deepseek_test.go`, `provider/deepseek/live_test.go`
- `Makefile` (`make live`), `docs/providers.md`, `docs/architecture.md`, `docs/roadmap.md`, `README.md`

## Tests added

`httptest` server replaying the vendor's SSE format; no network in default tests.

- Text + reasoning + usage (including cache-hit and reasoning tokens), keep-alive lines, auth header, `stream_options`.
- Parallel tool calls with arguments fragmented across chunks and interleaved indexes; dotted names round-trip.
- Exact normalized event sequence for a streamed tool call.
- Exact request body: system/user/assistant/tool mapping, reasoning echo, multiple tool results expanded, tool name mapping, schema, named tool choice, max tokens, temperature.
- Reasoning echo on tool-call turns without prior reasoning; error tool results flagged to the model; empty schemas filled in.
- Tool-name collisions, invalid requests and unsupported image inputs rejected with zero HTTP calls.
- Eleven HTTP error cases classified, with `Retry-After` and no leak of vendor text or keys.
- Four stream failure modes; finish-reason mapping.
- Consumer break and context cancellation both release the HTTP request (server observes it).
- Capabilities do not over-claim.
- `message`: reasoning validation, JSON round trip, `Reasoning()`; `provider`: reasoning deltas merged.
- Live tests (`-tags live`): list models, streamed text, and a full tool round trip that exercises the reasoning echo.

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
ok  	github.com/ynxdeiv/goodeiv/provider/deepseek
ok  	github.com/ynxdeiv/goodeiv/provider/providertest
ok  	github.com/ynxdeiv/goodeiv/usage

$ go test -count=1 -coverpkg=./internal/chatcompletions,./provider/deepseek ./provider/deepseek
ok  	github.com/ynxdeiv/goodeiv/provider/deepseek	coverage: 88.3% of statements in ./internal/chatcompletions, ./provider/deepseek

$ make live    (DEEPSEEK_MODEL=deepseek-flash)
live_test.go: model: deepseek-flash
live_test.go: model: deepseek-v4-pro
--- PASS: TestLiveListModels (0.63s)
live_test.go: text="pong" finish=stop model=deepseek-flash reasoning_chars=101 input=38 output=27
--- PASS: TestLiveStreamText (0.83s)
live_test.go: tool call: weather.current {"city": "Lisbon"} (reasoning_chars=85)
live_test.go: answer: "Right now in Lisbon it's **22 °C and sunny** ☀️ ..." finish=stop
--- PASS: TestLiveToolRoundTrip (2.47s)

$ DEEPSEEK_MODEL=deepseek-v4-pro go test -tags live -run TestLiveToolRoundTrip ./provider/deepseek -v
--- PASS: TestLiveToolRoundTrip (3.66s)

tool_choice probe against the live API (deepseek-flash):
required -> HTTP 400 Thinking mode does not support this tool_choice
named    -> HTTP 400 Thinking mode does not support this tool_choice   (both models)
none     -> HTTP 200
auto     -> HTTP 200
```

## Result

DeepSeek adapter complete, green offline and verified against the live API on both models,
including a full tool round trip that depends on the reasoning echo.

## Remaining risks

- The vendor may start accepting forced tool choices; flipping `ForcedToolChoice` is then a one-line change.
- `internal/chatcompletions` only supports text content; image parts for vision-capable compatible
  vendors come with the first such adapter.

## Next step

Phase `04-tool-contract`.
