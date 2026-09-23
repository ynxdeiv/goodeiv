// Package usage models token consumption reported by LLM providers.
//
// Providers report different subsets of counters. A Count distinguishes a value the provider
// did not report from a reported zero, so nothing is ever invented.
package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Count is a token counter that may be unknown. The zero Count is unknown.
type Count struct {
	value int64
	known bool
}

// Known returns a Count holding n.
func Known(n int64) Count {
	return Count{value: n, known: true}
}

// Value returns the counter and whether it was reported.
func (c Count) Value() (int64, bool) {
	return c.value, c.known
}

// IsKnown reports whether the counter was reported.
func (c Count) IsKnown() bool {
	return c.known
}

// Add sums the known values of c and other. The result is unknown only if both are unknown.
func (c Count) Add(other Count) Count {
	if !c.known && !other.known {
		return Count{}
	}
	return Known(c.value + other.value)
}

// MarshalJSON encodes a known count as a number and an unknown count as null.
func (c Count) MarshalJSON() ([]byte, error) {
	if !c.known {
		return []byte("null"), nil
	}
	return json.Marshal(c.value)
}

// UnmarshalJSON decodes a number into a known count and null into an unknown count.
func (c *Count) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*c = Count{}
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("usage: decoding count: %w", err)
	}
	*c = Known(n)
	return nil
}

// Usage is the token consumption of one or more model calls.
type Usage struct {
	// InputTokens counts prompt tokens, including cached ones.
	InputTokens Count `json:"input_tokens,omitzero"`
	// OutputTokens counts generated tokens, including reasoning ones.
	OutputTokens Count `json:"output_tokens,omitzero"`
	// CachedInputTokens counts input tokens served from the provider's prompt cache.
	CachedInputTokens Count `json:"cached_input_tokens,omitzero"`
	// CacheWriteTokens counts input tokens written to the provider's prompt cache.
	CacheWriteTokens Count `json:"cache_write_tokens,omitzero"`
	// ReasoningTokens counts output tokens spent on hidden reasoning.
	ReasoningTokens Count `json:"reasoning_tokens,omitzero"`
}

// Add returns the field-by-field sum of u and other.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:       u.InputTokens.Add(other.InputTokens),
		OutputTokens:      u.OutputTokens.Add(other.OutputTokens),
		CachedInputTokens: u.CachedInputTokens.Add(other.CachedInputTokens),
		CacheWriteTokens:  u.CacheWriteTokens.Add(other.CacheWriteTokens),
		ReasoningTokens:   u.ReasoningTokens.Add(other.ReasoningTokens),
	}
}

// TotalTokens returns input plus output tokens.
func (u Usage) TotalTokens() Count {
	return u.InputTokens.Add(u.OutputTokens)
}
