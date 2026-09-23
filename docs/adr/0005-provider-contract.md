# 0005 — Provider contract and streaming

- Status: Accepted
- Date: 2026-09-22

## Context

Every vendor exposes a blocking call and a streaming call with different event shapes. The runtime
needs one contract that makes cancellation and resource cleanup hard to get wrong, keeps adapters
small and lets tests run without a network.

## Decision

- `provider.Provider` has three methods: `Generate`, `Stream` and `Capabilities(model)`.
- `Stream` returns `iter.Seq2[Event, error]`. Breaking out of the loop stops the stream, and the
  adapter's iterator function returns, which is where it closes its HTTP body. No channels, no
  background goroutines, no `Close` to forget.
- Events are normalized (`text_delta`, `reasoning_delta`, `tool_call_started`, `tool_call_delta`,
  `tool_call_completed`, `usage`, `completed`). A failure is a yielded error and ends the stream;
  a stream that ends without `completed` is `stream_failed`, never a silent success.
- `provider.Collect` assembles a `Response` from any stream, so adapters implement streaming once
  and derive `Generate` from it.
- `Request.Validate` rejects malformed requests before tokens are spent, including tool results
  that answer no earlier tool call. `Capabilities.Supports` rejects inputs a model cannot handle.
- `providertest` offers a scripted provider for deterministic, network-free tests.

## Alternatives considered

- **Channel of chunks**: needs a goroutine per stream and makes early exit leak-prone.
- **`Next()/Event()/Close()` iterator interface**: works, but relies on callers remembering `Close`.
- **Separate `Generator` and `Streamer` interfaces**: every useful adapter supports both, and
  `Collect` makes `Generate` nearly free, so splitting only adds ceremony.

## Consequences

- Requires Go 1.23+ range-over-func (the module targets Go 1.26).
- Adapters must emit `tool_call_completed` for every started call and end with `completed`.
