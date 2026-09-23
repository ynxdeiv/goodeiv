package chatcompletions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ynxdeiv/goodeiv/fault"
)

const maxErrorBodyBytes = 64 << 10

type vendorError struct {
	status  int
	message string
	code    string
}

func (e vendorError) Error() string {
	return fmt.Sprintf("vendor error (status %d, code %s): %s", e.status, e.code, e.message)
}

func statusError(name string, resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	cause := vendorError{status: resp.StatusCode, message: strings.TrimSpace(string(raw))}
	var envelope chatErrorEnvelope
	if json.Unmarshal(raw, &envelope) == nil && envelope.Error != nil {
		cause.message = envelope.Error.Message
		if envelope.Error.Code != nil {
			cause.code = fmt.Sprint(envelope.Error.Code)
		}
	}

	err := fault.Wrap(classify(cause), fmt.Sprintf("%s: request failed with HTTP %d", name, resp.StatusCode), cause)
	if delay, ok := retryAfter(resp.Header.Get("Retry-After")); ok {
		err = err.WithRetryAfter(delay)
	}
	return err
}

func classify(e vendorError) fault.Kind {
	switch e.status {
	case http.StatusBadRequest:
		if mentionsContextLength(e.message) {
			return fault.ContextTooLarge
		}
		return fault.InvalidInput
	case http.StatusRequestEntityTooLarge:
		return fault.ContextTooLarge
	case http.StatusUnauthorized:
		return fault.Unauthenticated
	case http.StatusPaymentRequired:
		return fault.QuotaExhausted
	case http.StatusForbidden:
		return fault.Unauthorized
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return fault.InvalidInput
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return fault.Timeout
	case http.StatusTooManyRequests:
		if e.code == "insufficient_quota" {
			return fault.QuotaExhausted
		}
		return fault.RateLimited
	case http.StatusServiceUnavailable, 529:
		return fault.ProviderUnavailable
	}
	if e.status >= 500 {
		return fault.TemporaryFailure
	}
	return fault.Internal
}

func mentionsContextLength(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "context length") || strings.Contains(lower, "context_length")
}

func retryAfter(header string) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(header); err == nil {
		if delay := time.Until(at); delay > 0 {
			return delay, true
		}
	}
	return 0, false
}

func transportError(ctx context.Context, name string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fault.Wrap(fault.Timeout, name+": provider did not respond in time", err)
	}
	return fault.Wrap(fault.TemporaryFailure, name+": could not reach provider", err)
}
