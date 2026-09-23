package provider_test

import (
	"encoding/json"
	"errors"
	"iter"
	"reflect"
	"testing"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/usage"
)

func events(evs ...provider.Event) iter.Seq2[provider.Event, error] {
	return func(yield func(provider.Event, error) bool) {
		for _, ev := range evs {
			if !yield(ev, nil) {
				return
			}
		}
	}
}

func failing(err error, evs ...provider.Event) iter.Seq2[provider.Event, error] {
	return func(yield func(provider.Event, error) bool) {
		for _, ev := range evs {
			if !yield(ev, nil) {
				return
			}
		}
		yield(provider.Event{}, err)
	}
}

func TestCollectText(t *testing.T) {
	resp, err := provider.Collect(events(
		provider.Event{Type: provider.EventTextDelta, Delta: "Hel"},
		provider.Event{Type: provider.EventTextDelta, Delta: "lo"},
		provider.Event{Type: provider.EventReasoningDelta, Delta: "thinking"},
		provider.Event{Type: provider.EventCompleted, FinishReason: provider.FinishStop, Model: "m-1", RequestID: "req-1"},
	))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want := provider.Response{
		Message:      message.Assistant("Hello"),
		FinishReason: provider.FinishStop,
		Model:        "m-1",
		RequestID:    "req-1",
	}
	if !reflect.DeepEqual(resp, want) {
		t.Fatalf("got %+v\nwant %+v", resp, want)
	}
}

func TestCollectPreservesPartOrder(t *testing.T) {
	call := message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{"city":"Lisbon"}`)}

	resp, err := provider.Collect(events(
		provider.Event{Type: provider.EventTextDelta, Delta: "Let me check."},
		provider.Event{Type: provider.EventToolCallStarted, ToolCall: message.ToolCall{ID: "c1", Name: "weather.current"}},
		provider.Event{Type: provider.EventToolCallDelta, ToolCall: message.ToolCall{ID: "c1"}, Delta: `{"city":`},
		provider.Event{Type: provider.EventToolCallDelta, ToolCall: message.ToolCall{ID: "c1"}, Delta: `"Lisbon"}`},
		provider.Event{Type: provider.EventToolCallCompleted, ToolCall: call},
		provider.Event{Type: provider.EventTextDelta, Delta: "Done."},
		provider.Event{Type: provider.EventCompleted, FinishReason: provider.FinishToolCalls},
	))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want := message.New(message.RoleAssistant, message.Text{Text: "Let me check."}, call, message.Text{Text: "Done."})
	if !reflect.DeepEqual(resp.Message, want) {
		t.Fatalf("message = %+v\nwant %+v", resp.Message, want)
	}
}

func TestCollectForcesToolCallsFinishReason(t *testing.T) {
	resp, err := provider.Collect(events(
		provider.Event{Type: provider.EventToolCallCompleted, ToolCall: message.ToolCall{ID: "c1", Name: "x"}},
		provider.Event{Type: provider.EventCompleted, FinishReason: provider.FinishStop},
	))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if resp.FinishReason != provider.FinishToolCalls {
		t.Fatalf("FinishReason = %q, want tool_calls", resp.FinishReason)
	}
}

func TestCollectMergesUsageSnapshots(t *testing.T) {
	resp, err := provider.Collect(events(
		provider.Event{Type: provider.EventUsage, Usage: usage.Usage{InputTokens: usage.Known(100), OutputTokens: usage.Known(1)}},
		provider.Event{Type: provider.EventTextDelta, Delta: "ok"},
		provider.Event{Type: provider.EventUsage, Usage: usage.Usage{OutputTokens: usage.Known(12)}},
		provider.Event{Type: provider.EventCompleted, FinishReason: provider.FinishStop},
	))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want := usage.Usage{InputTokens: usage.Known(100), OutputTokens: usage.Known(12)}
	if resp.Usage != want {
		t.Fatalf("usage = %+v, want %+v", resp.Usage, want)
	}
}

func TestCollectFailures(t *testing.T) {
	boom := fault.New(fault.ProviderUnavailable, "overloaded")

	tests := []struct {
		name     string
		stream   iter.Seq2[provider.Event, error]
		wantKind fault.Kind
		wantErr  error
	}{
		{"stream error is returned as is", failing(boom, provider.Event{Type: provider.EventTextDelta, Delta: "par"}), fault.ProviderUnavailable, boom},
		{"missing completion", events(provider.Event{Type: provider.EventTextDelta, Delta: "cut"}), fault.StreamFailed, nil},
		{"started tool call never completed", events(
			provider.Event{Type: provider.EventToolCallStarted, ToolCall: message.ToolCall{ID: "c1", Name: "x"}},
			provider.Event{Type: provider.EventCompleted, FinishReason: provider.FinishToolCalls},
		), fault.StreamFailed, nil},
		{"events after completion", events(
			provider.Event{Type: provider.EventCompleted, FinishReason: provider.FinishStop},
			provider.Event{Type: provider.EventTextDelta, Delta: "late"},
		), fault.StreamFailed, nil},
		{"unknown event type", events(provider.Event{Type: "mystery"}), fault.StreamFailed, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.Collect(tt.stream)
			if got := fault.KindOf(err); got != tt.wantKind {
				t.Fatalf("kind = %q, want %q (err %v)", got, tt.wantKind, err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v to wrap %v", err, tt.wantErr)
			}
		})
	}
}

func TestFinishReasonValid(t *testing.T) {
	for _, reason := range []provider.FinishReason{
		provider.FinishStop, provider.FinishLength, provider.FinishToolCalls,
		provider.FinishContentFilter, provider.FinishOther,
	} {
		if !reason.Valid() {
			t.Fatalf("%q should be valid", reason)
		}
	}
	if provider.FinishReason("").Valid() || provider.FinishReason("banana").Valid() {
		t.Fatal("empty and unknown reasons must be invalid")
	}
}
