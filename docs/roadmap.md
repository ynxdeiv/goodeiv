# Roadmap

Each phase = 1 branch = 1 PR, with evidence in `docs/qa/phases/<id>.md`.
The order may change; record the reason here when it does.

- Finish reason moved from 01 to 02: it describes a provider response, not a message.
- Vertical slice first: a real read-only integration (06) lands right after the runtime loop, so the
  design is validated end to end early. Mutating integration tools (10) wait for references, approvals
  and idempotency (08–09), because they must never run without those guarantees.

| # | Phase | Deliverable | Status |
|---|---|---|---|
| 00 | `00-foundation` | Repository layout, tooling, gates, docs | done |
| 01 | `01-core-types` | `message` (roles, typed parts, trust), `usage`, `fault` error taxonomy | done |
| 02 | `02-provider-contract` | provider contract, request/response, finish reason, stream events, capabilities, registry, scripted fake provider | pending |
| 03 | `03-provider-openai` | first real adapter (generate + stream) | pending |
| 04 | `04-tool-contract` | tool, definition, result, risk, registry, fake tools | pending |
| 05 | `05-runtime-loop` | bounded loop, execution context, cancellation, timeouts | pending |
| 06 | `06-integration-google-read` | first module under `integrations/`: read-only Gmail and Drive tools | pending |
| 07 | `07-retry-fallback` | retry with backoff/jitter/Retry-After, model fallback | pending |
| 08 | `08-references` | reference, ReferenceStore, in-memory implementation | pending |
| 09 | `09-hitl-policy` | policy, approval, ApprovalStore, idempotency | pending |
| 10 | `10-integration-google-write` | mutating Google tools (send, create, delete) behind approvals | pending |
| 11 | `11-observability` | events, trace IDs, aggregated usage | pending |
| 12 | `12-structured-output` | typed generation with validation | pending |
| 13 | `13-providers-more` | Anthropic, Gemini and other adapters | pending |
