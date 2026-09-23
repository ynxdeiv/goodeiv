# Errors

Every goodeiv error can be classified with the `fault` package. See [ADR 0004](adr/0004-error-model.md).

```go
result, err := rt.Run(ctx, req)
switch {
case errors.Is(err, fault.ApprovalRequired):
	askHuman(result)
case fault.IsRetryable(err):
	delay, _ := fault.RetryAfter(err)
	scheduleRetry(delay)
case err != nil:
	log.Printf("run failed: %s", err)
}
```

## Kinds

| Kind | Meaning | Retryable |
|---|---|---|
| `invalid_input` | Malformed request, message or configuration | no |
| `unauthenticated` | Missing or rejected credentials | no |
| `unauthorized` | Authenticated but not allowed | no |
| `rate_limited` | Upstream asked to slow down | **yes** |
| `quota_exhausted` | Billing or usage quota exhausted — waiting will not help | no |
| `timeout` | Upstream did not answer in time | **yes** |
| `canceled` | The caller canceled the operation | no |
| `provider_unavailable` | Provider is down or overloaded | **yes** |
| `temporary_failure` | Transient upstream or network failure | **yes** |
| `context_too_large` | Input exceeds the model's context window | no |
| `invalid_tool_call` | The model produced an unusable tool call | no |
| `tool_not_found` | The model called a tool that is not registered or allowed | no |
| `tool_execution_failed` | The tool ran and failed | no |
| `approval_required` | Execution paused waiting for a human decision | no |
| `approval_rejected` | A human rejected the action | no |
| `unsafe_action` | A policy blocked the action | no |
| `reference_not_found` | Unknown reference ID | no |
| `reference_expired` | Reference TTL elapsed | no |
| `reference_forbidden` | Reference belongs to another user or organization | no |
| `max_steps_exceeded` | Run hit its step limit | no |
| `max_tool_calls_exceeded` | Run hit its tool-call limit | no |
| `structured_output_invalid` | Model output failed schema validation | no |
| `stream_failed` | A stream broke after it started | no |
| `internal` | Unclassified error | no |

## Rules

- `err.Error()` never contains the raw cause. Use `errors.Unwrap` or `errors.As(err, &fe)` and read
  `fe.Err` when debugging, and sanitize before logging.
- Context cancellation and deadlines of the **caller's** context are never retryable.
- Adapters and tools must map their failures to a kind; unmapped errors surface as `internal`.
