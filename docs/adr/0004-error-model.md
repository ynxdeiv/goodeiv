# 0004 — Error model

- Status: Accepted
- Date: 2026-09-22

## Context

Callers must react to failures (retry, ask for approval, show a message) without parsing strings
or knowing which provider or tool failed. Raw vendor errors often embed request payloads, account
details or partial credentials and must not reach logs or end users verbatim.

## Decision

- A single closed taxonomy, `fault.Kind`, shared by every package.
- `fault.Kind` implements `error`, so `errors.Is(err, fault.RateLimited)` works on any wrapped error.
- `*fault.Error` carries `Kind`, a caller-safe `Message`, an optional `RetryAfter` and the raw cause in `Err`.
- `Error()` prints only kind and message. The cause is reachable through `errors.Unwrap`/`errors.As`
  for debugging, but never printed by default.
- Retryability is a property of the kind (`rate_limited`, `timeout`, `provider_unavailable`,
  `temporary_failure`). Cancellation or expiry of the caller's own context is never retryable.
- `quota_exhausted` is separate from `rate_limited`: some providers answer HTTP 429 for exhausted
  billing quota, and retrying those only wastes time.

## Alternatives considered

- **Sentinel errors per package**: callers would need to know every package's sentinels.
- **Printing the cause in `Error()`**: convenient for debugging, but leaks vendor payloads into logs.
- **HTTP-status-based classification**: ties the model to HTTP and loses meaning for tools and policies.

## Consequences

- Adding a kind is a minor, additive change; removing or renaming one is breaking.
- Debug logging that needs the cause must unwrap explicitly and is responsible for sanitizing it.
