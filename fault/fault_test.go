package fault_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ynxdeiv/goodeiv/fault"
)

func TestErrorMatchesKindWithErrorsIs(t *testing.T) {
	err := fmt.Errorf("calling provider: %w", fault.New(fault.RateLimited, "too many requests"))

	if !errors.Is(err, fault.RateLimited) {
		t.Fatal("expected errors.Is to match fault.RateLimited")
	}
	if errors.Is(err, fault.Timeout) {
		t.Fatal("expected errors.Is not to match a different kind")
	}
}

func TestErrorPreservesCauseWithoutExposingIt(t *testing.T) {
	cause := errors.New(`upstream said: {"error":"invalid x-api-key sk-secret"}`)
	err := fault.Wrap(fault.Unauthenticated, "provider rejected credentials", cause)

	if !errors.Is(err, cause) {
		t.Fatal("expected the cause to be reachable through errors.Is")
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error message leaks the raw cause: %q", err.Error())
	}
	if got, want := err.Error(), "unauthenticated: provider rejected credentials"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestErrorsAsExposesDetails(t *testing.T) {
	err := fmt.Errorf("step 3: %w", fault.New(fault.RateLimited, "slow down").WithRetryAfter(2*time.Second))

	var fe *fault.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected errors.As to find *fault.Error")
	}
	if fe.Kind != fault.RateLimited || fe.RetryAfter != 2*time.Second {
		t.Fatalf("unexpected details: %+v", fe)
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want fault.Kind
	}{
		{"nil", nil, ""},
		{"fault error", fault.New(fault.ToolNotFound, "missing"), fault.ToolNotFound},
		{"wrapped fault error", fmt.Errorf("x: %w", fault.New(fault.ContextTooLarge, "big")), fault.ContextTooLarge},
		{"bare kind", fault.ApprovalRequired, fault.ApprovalRequired},
		{"context canceled", context.Canceled, fault.Canceled},
		{"context deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), fault.Timeout},
		{"unknown error", errors.New("boom"), fault.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fault.KindOf(tt.err); got != tt.want {
				t.Fatalf("KindOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"rate limited", fault.New(fault.RateLimited, ""), true},
		{"provider timeout", fault.New(fault.Timeout, ""), true},
		{"provider unavailable", fault.New(fault.ProviderUnavailable, ""), true},
		{"temporary failure", fault.New(fault.TemporaryFailure, ""), true},
		{"quota exhausted", fault.New(fault.QuotaExhausted, ""), false},
		{"invalid input", fault.New(fault.InvalidInput, ""), false},
		{"unauthenticated", fault.New(fault.Unauthenticated, ""), false},
		{"unauthorized", fault.New(fault.Unauthorized, ""), false},
		{"unsafe action", fault.New(fault.UnsafeAction, ""), false},
		{"approval rejected", fault.New(fault.ApprovalRejected, ""), false},
		{"caller canceled", context.Canceled, false},
		{"caller deadline", context.DeadlineExceeded, false},
		{"unknown", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fault.IsRetryable(tt.err); got != tt.want {
				t.Fatalf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryAfter(t *testing.T) {
	if _, ok := fault.RetryAfter(errors.New("boom")); ok {
		t.Fatal("expected no retry-after for a plain error")
	}
	if _, ok := fault.RetryAfter(fault.New(fault.RateLimited, "")); ok {
		t.Fatal("expected no retry-after when it was not set")
	}
	d, ok := fault.RetryAfter(fmt.Errorf("x: %w", fault.New(fault.RateLimited, "").WithRetryAfter(time.Second)))
	if !ok || d != time.Second {
		t.Fatalf("RetryAfter() = %v, %v; want 1s, true", d, ok)
	}
}

func TestWithRetryAfterDoesNotMutateOriginal(t *testing.T) {
	base := fault.New(fault.RateLimited, "")
	_ = base.WithRetryAfter(time.Minute)

	if base.RetryAfter != 0 {
		t.Fatal("WithRetryAfter mutated the receiver")
	}
}

func TestKindValid(t *testing.T) {
	if !fault.StreamFailed.Valid() {
		t.Fatal("expected a declared kind to be valid")
	}
	if fault.Kind("made_up").Valid() {
		t.Fatal("expected an undeclared kind to be invalid")
	}
}

func ExampleKindOf() {
	err := fmt.Errorf("running agent: %w", fault.New(fault.MaxStepsExceeded, "limit of 8 steps reached"))

	fmt.Println(fault.KindOf(err))
	fmt.Println(errors.Is(err, fault.MaxStepsExceeded))
	// Output:
	// max_steps_exceeded
	// true
}
