// Package fault defines the normalized error taxonomy shared by every goodeiv package.
//
// Errors carry a Kind that callers can match with errors.Is, while the original cause
// stays reachable through errors.Unwrap without leaking into Error().
package fault

import (
	"context"
	"errors"
	"time"
)

// Kind classifies an error. It implements error so it can be used as an errors.Is target.
type Kind string

// Error kinds.
const (
	InvalidInput            Kind = "invalid_input"
	Unauthenticated         Kind = "unauthenticated"
	Unauthorized            Kind = "unauthorized"
	RateLimited             Kind = "rate_limited"
	QuotaExhausted          Kind = "quota_exhausted"
	Timeout                 Kind = "timeout"
	Canceled                Kind = "canceled"
	ProviderUnavailable     Kind = "provider_unavailable"
	TemporaryFailure        Kind = "temporary_failure"
	ContextTooLarge         Kind = "context_too_large"
	InvalidToolCall         Kind = "invalid_tool_call"
	ToolNotFound            Kind = "tool_not_found"
	ToolExecutionFailed     Kind = "tool_execution_failed"
	ApprovalRequired        Kind = "approval_required"
	ApprovalRejected        Kind = "approval_rejected"
	UnsafeAction            Kind = "unsafe_action"
	ReferenceNotFound       Kind = "reference_not_found"
	ReferenceExpired        Kind = "reference_expired"
	ReferenceForbidden      Kind = "reference_forbidden"
	MaxStepsExceeded        Kind = "max_steps_exceeded"
	MaxToolCallsExceeded    Kind = "max_tool_calls_exceeded"
	StructuredOutputInvalid Kind = "structured_output_invalid"
	StreamFailed            Kind = "stream_failed"
	Internal                Kind = "internal"
)

var validKinds = map[Kind]bool{
	InvalidInput: true, Unauthenticated: true, Unauthorized: true, RateLimited: true,
	QuotaExhausted: true, Timeout: true, Canceled: true, ProviderUnavailable: true,
	TemporaryFailure: true, ContextTooLarge: true, InvalidToolCall: true, ToolNotFound: true,
	ToolExecutionFailed: true, ApprovalRequired: true, ApprovalRejected: true, UnsafeAction: true,
	ReferenceNotFound: true, ReferenceExpired: true, ReferenceForbidden: true,
	MaxStepsExceeded: true, MaxToolCallsExceeded: true, StructuredOutputInvalid: true,
	StreamFailed: true, Internal: true,
}

var retryableKinds = map[Kind]bool{
	RateLimited:         true,
	Timeout:             true,
	ProviderUnavailable: true,
	TemporaryFailure:    true,
}

// Error returns the kind itself, so a bare Kind is a valid error value.
func (k Kind) Error() string {
	return string(k)
}

// Valid reports whether k is one of the declared kinds.
func (k Kind) Valid() bool {
	return validKinds[k]
}

// Retryable reports whether an operation that failed with k may succeed if attempted again.
func (k Kind) Retryable() bool {
	return retryableKinds[k]
}

// Error is a classified error. Message is safe to show to callers; Err keeps the raw cause.
type Error struct {
	// Kind classifies the failure.
	Kind Kind
	// Message is a caller-safe description. It must not contain secrets or raw vendor payloads.
	Message string
	// RetryAfter is the delay suggested by the upstream service, or zero when unknown.
	RetryAfter time.Duration
	// Err is the underlying cause, available through errors.Unwrap but never printed by Error.
	Err error
}

// New returns an Error of the given kind.
func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// Wrap returns an Error of the given kind that keeps cause reachable through errors.Unwrap.
func Wrap(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, Err: cause}
}

// WithRetryAfter returns a copy of e with RetryAfter set to d.
func (e *Error) WithRetryAfter(d time.Duration) *Error {
	clone := *e
	clone.RetryAfter = d
	return &clone
}

// Error returns the kind followed by the caller-safe message. The cause is omitted on purpose.
func (e *Error) Error() string {
	if e.Message == "" {
		return string(e.Kind)
	}
	return string(e.Kind) + ": " + e.Message
}

// Unwrap returns the underlying cause.
func (e *Error) Unwrap() error {
	return e.Err
}

// Is reports whether target is the Kind of e, enabling errors.Is(err, fault.RateLimited).
func (e *Error) Is(target error) bool {
	kind, ok := target.(Kind)
	return ok && kind == e.Kind
}

// KindOf returns the Kind of err. Context cancellation maps to Canceled, context deadlines
// to Timeout, unclassified errors to Internal and nil to the empty Kind.
func KindOf(err error) Kind {
	if err == nil {
		return ""
	}
	var fe *Error
	if errors.As(err, &fe) {
		return fe.Kind
	}
	var kind Kind
	if errors.As(err, &kind) {
		return kind
	}
	switch {
	case errors.Is(err, context.Canceled):
		return Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return Timeout
	}
	return Internal
}

// IsRetryable reports whether err is worth retrying. Cancellation or expiry of the caller's
// own context is never retryable, even though a provider-side Timeout is.
func IsRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return KindOf(err).Retryable()
}

// RetryAfter returns the upstream retry delay carried by err, if any.
func RetryAfter(err error) (time.Duration, bool) {
	var fe *Error
	if errors.As(err, &fe) && fe.RetryAfter > 0 {
		return fe.RetryAfter, true
	}
	return 0, false
}
