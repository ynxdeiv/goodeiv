package usage_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ynxdeiv/goodeiv/usage"
)

func TestCountDistinguishesUnknownFromZero(t *testing.T) {
	var unknown usage.Count
	if _, ok := unknown.Value(); ok {
		t.Fatal("zero Count must be unknown")
	}

	zero := usage.Known(0)
	if v, ok := zero.Value(); !ok || v != 0 {
		t.Fatalf("Known(0).Value() = %d, %v; want 0, true", v, ok)
	}
}

func TestCountAdd(t *testing.T) {
	tests := []struct {
		name      string
		a, b      usage.Count
		want      int64
		wantKnown bool
	}{
		{"both known", usage.Known(3), usage.Known(4), 7, true},
		{"left unknown", usage.Count{}, usage.Known(4), 4, true},
		{"right unknown", usage.Known(3), usage.Count{}, 3, true},
		{"both unknown", usage.Count{}, usage.Count{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := tt.a.Add(tt.b).Value()
			if got != tt.want || known != tt.wantKnown {
				t.Fatalf("Add() = %d, %v; want %d, %v", got, known, tt.want, tt.wantKnown)
			}
		})
	}
}

func TestUsageAddAggregatesEveryField(t *testing.T) {
	first := usage.Usage{
		InputTokens:       usage.Known(100),
		OutputTokens:      usage.Known(20),
		CachedInputTokens: usage.Known(80),
	}
	second := usage.Usage{
		InputTokens:     usage.Known(50),
		OutputTokens:    usage.Known(10),
		ReasoningTokens: usage.Known(5),
	}

	total := first.Add(second)

	assertCount(t, "input", total.InputTokens, 150, true)
	assertCount(t, "output", total.OutputTokens, 30, true)
	assertCount(t, "cached", total.CachedInputTokens, 80, true)
	assertCount(t, "reasoning", total.ReasoningTokens, 5, true)
	assertCount(t, "cache write", total.CacheWriteTokens, 0, false)
}

func TestUsageTotalTokens(t *testing.T) {
	u := usage.Usage{InputTokens: usage.Known(10), OutputTokens: usage.Known(5)}
	assertCount(t, "total", u.TotalTokens(), 15, true)

	assertCount(t, "empty total", usage.Usage{}.TotalTokens(), 0, false)
}

func TestUsageJSONKeepsUnknownFieldsAbsent(t *testing.T) {
	u := usage.Usage{InputTokens: usage.Known(10), OutputTokens: usage.Known(0)}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(data), `{"input_tokens":10,"output_tokens":0}`; got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}

	var decoded usage.Usage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded != u {
		t.Fatalf("round trip mismatch: %+v != %+v", decoded, u)
	}
}

func TestCountJSONNull(t *testing.T) {
	var c usage.Count
	if err := json.Unmarshal([]byte("null"), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := c.Value(); ok {
		t.Fatal("null must decode to an unknown count")
	}
	if err := json.Unmarshal([]byte(`"x"`), &c); err == nil {
		t.Fatal("expected an error for a non-numeric count")
	}
}

func assertCount(t *testing.T, name string, c usage.Count, want int64, wantKnown bool) {
	t.Helper()
	got, known := c.Value()
	if got != want || known != wantKnown {
		t.Fatalf("%s = %d, %v; want %d, %v", name, got, known, want, wantKnown)
	}
}

func ExampleUsage_Add() {
	step1 := usage.Usage{InputTokens: usage.Known(1200), OutputTokens: usage.Known(80)}
	step2 := usage.Usage{InputTokens: usage.Known(1350), OutputTokens: usage.Known(40)}

	total := step1.Add(step2)
	tokens, _ := total.TotalTokens().Value()
	_, cachedKnown := total.CachedInputTokens.Value()

	fmt.Println(tokens)
	fmt.Println(cachedKnown)
	// Output:
	// 2670
	// false
}
