# Roadmap

Each phase = 1 branch = 1 PR, with evidence in `docs/qa/phases/<id>.md`.
The order may change; record the reason here when it does.

| # | Phase | Deliverable | Status |
|---|---|---|---|
| 00 | `00-foundation` | Repository layout, tooling, gates, docs | in progress |
| 01 | `01-core-types` | message, content, role, usage, finish reason, error taxonomy | pending |
| 02 | `02-provider-contract` | provider contract, capabilities, registry, fake provider | pending |
| 03 | `03-provider-openai` | first real adapter (generate + stream) | pending |
| 04 | `04-tool-contract` | tool, definition, result, risk, registry, fake tools | pending |
| 05 | `05-runtime-loop` | bounded loop, execution context, cancellation, timeouts | pending |
| 06 | `06-retry-fallback` | retry with backoff/jitter/Retry-After, model fallback | pending |
| 07 | `07-references` | reference, ReferenceStore, in-memory implementation | pending |
| 08 | `08-hitl-policy` | policy, approval, ApprovalStore, idempotency | pending |
| 09 | `09-observability` | events, trace IDs, aggregated usage | pending |
| 10 | `10-structured-output` | typed generation with validation | pending |
| 11 | `11-providers-more` | Anthropic, Gemini and other adapters | pending |
| 12 | `12-integration-google` | first module under `integrations/` | pending |
