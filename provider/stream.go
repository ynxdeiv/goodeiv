package provider

import (
	"iter"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/usage"
)

// Response is the complete result of a model call.
type Response struct {
	// Message is the assistant message, possibly containing tool calls.
	Message message.Message
	// FinishReason explains why generation stopped.
	FinishReason FinishReason
	// Usage is the token consumption reported by the provider.
	Usage usage.Usage
	// Model is the model that actually served the request, when reported.
	Model string
	// RequestID is the provider's request identifier, useful when contacting vendor support.
	RequestID string
}

// FinishReason explains why the model stopped generating.
type FinishReason string

// Finish reasons.
const (
	FinishStop          FinishReason = "stop"
	FinishLength        FinishReason = "length"
	FinishToolCalls     FinishReason = "tool_calls"
	FinishContentFilter FinishReason = "content_filter"
	FinishOther         FinishReason = "other"
)

// Valid reports whether r is one of the declared finish reasons.
func (r FinishReason) Valid() bool {
	switch r {
	case FinishStop, FinishLength, FinishToolCalls, FinishContentFilter, FinishOther:
		return true
	}
	return false
}

// EventType identifies a stream event.
type EventType string

// Stream event types.
const (
	EventTextDelta         EventType = "text_delta"
	EventReasoningDelta    EventType = "reasoning_delta"
	EventToolCallStarted   EventType = "tool_call_started"
	EventToolCallDelta     EventType = "tool_call_delta"
	EventToolCallCompleted EventType = "tool_call_completed"
	EventUsage             EventType = "usage"
	EventCompleted         EventType = "completed"
)

// Event is one normalized stream event. Which fields are set depends on Type:
//
//   - EventTextDelta, EventReasoningDelta: Delta. Consecutive deltas are merged into one
//     message.Text or message.Reasoning part.
//   - EventToolCallStarted: ToolCall.ID and ToolCall.Name.
//   - EventToolCallDelta: ToolCall.ID and an argument fragment in Delta.
//   - EventToolCallCompleted: the full ToolCall. Every started call must complete.
//   - EventUsage: a usage snapshot; known fields replace earlier values.
//   - EventCompleted: FinishReason, Model and RequestID. It is always the last event.
type Event struct {
	// Type selects which other fields are meaningful.
	Type EventType
	// Delta is a text, reasoning or tool argument fragment.
	Delta string
	// ToolCall identifies the tool call the event refers to.
	ToolCall message.ToolCall
	// Usage is a usage snapshot.
	Usage usage.Usage
	// FinishReason is set on EventCompleted.
	FinishReason FinishReason
	// Model is set on EventCompleted when the provider reports it.
	Model string
	// RequestID is set on EventCompleted when the provider reports it.
	RequestID string
}

// Collect consumes a stream and assembles the Response. Stream errors are returned unchanged;
// protocol violations, such as a missing EventCompleted, are fault.StreamFailed. A response
// containing tool calls always reports FinishToolCalls, whatever the provider said.
func Collect(stream iter.Seq2[Event, error]) (Response, error) {
	var c collector
	for ev, err := range stream {
		if err != nil {
			return Response{}, err
		}
		if err := c.add(ev); err != nil {
			return Response{}, err
		}
	}
	return c.response()
}

type collector struct {
	resp      Response
	parts     []message.Part
	pending   map[string]bool
	completed bool
}

func (c *collector) add(ev Event) error {
	if c.completed {
		return streamFailed("event received after completion")
	}
	switch ev.Type {
	case EventTextDelta:
		c.parts = appendDelta(c.parts, ev.Delta, func(text string) message.Part { return message.Text{Text: text} })
	case EventReasoningDelta:
		c.parts = appendDelta(c.parts, ev.Delta, func(text string) message.Part { return message.Reasoning{Text: text} })
	case EventToolCallDelta:
	case EventToolCallStarted:
		if c.pending == nil {
			c.pending = map[string]bool{}
		}
		c.pending[ev.ToolCall.ID] = true
	case EventToolCallCompleted:
		delete(c.pending, ev.ToolCall.ID)
		c.parts = append(c.parts, ev.ToolCall)
	case EventUsage:
		c.resp.Usage = mergeSnapshot(c.resp.Usage, ev.Usage)
	case EventCompleted:
		c.completed = true
		c.resp.FinishReason = ev.FinishReason
		c.resp.Model = ev.Model
		c.resp.RequestID = ev.RequestID
	default:
		return streamFailed("unknown event type " + string(ev.Type))
	}
	return nil
}

func appendDelta(parts []message.Part, delta string, build func(string) message.Part) []message.Part {
	if delta == "" {
		return parts
	}
	next := build(delta)
	if n := len(parts); n > 0 {
		switch last := parts[n-1].(type) {
		case message.Text:
			if _, same := next.(message.Text); same {
				parts[n-1] = message.Text{Text: last.Text + delta}
				return parts
			}
		case message.Reasoning:
			if _, same := next.(message.Reasoning); same {
				parts[n-1] = message.Reasoning{Text: last.Text + delta}
				return parts
			}
		}
	}
	return append(parts, next)
}

func (c *collector) response() (Response, error) {
	if !c.completed {
		return Response{}, streamFailed("stream ended without a completion event")
	}
	if len(c.pending) > 0 {
		return Response{}, streamFailed("stream completed with unfinished tool calls")
	}
	c.resp.Message = message.New(message.RoleAssistant, c.parts...)
	if len(c.resp.Message.ToolCalls()) > 0 {
		c.resp.FinishReason = FinishToolCalls
	}
	return c.resp, nil
}

func mergeSnapshot(current, next usage.Usage) usage.Usage {
	pick := func(old, updated usage.Count) usage.Count {
		if updated.IsKnown() {
			return updated
		}
		return old
	}
	return usage.Usage{
		InputTokens:       pick(current.InputTokens, next.InputTokens),
		OutputTokens:      pick(current.OutputTokens, next.OutputTokens),
		CachedInputTokens: pick(current.CachedInputTokens, next.CachedInputTokens),
		CacheWriteTokens:  pick(current.CacheWriteTokens, next.CacheWriteTokens),
		ReasoningTokens:   pick(current.ReasoningTokens, next.ReasoningTokens),
	}
}

func streamFailed(reason string) error {
	return fault.New(fault.StreamFailed, "provider: "+reason)
}
