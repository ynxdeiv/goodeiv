package chatcompletions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/usage"
)

type streamParser struct {
	name   string
	names  toolNames
	calls  map[int]*pendingCall
	order  []int
	finish string
	id     string
	model  string
}

type pendingCall struct {
	id        string
	name      string
	arguments strings.Builder
	completed bool
}

func newStreamParser(name string, names toolNames) *streamParser {
	return &streamParser{name: name, names: names, calls: map[int]*pendingCall{}}
}

func (p *streamParser) line(line string) ([]provider.Event, bool, error) {
	data, ok := strings.CutPrefix(line, "data:")
	if !ok {
		return nil, false, nil
	}
	data = strings.TrimSpace(data)
	if data == "[DONE]" {
		events, err := p.done()
		return events, true, err
	}

	var chunk chatChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, false, fault.Wrap(fault.StreamFailed, p.name+": malformed stream chunk", err)
	}
	if chunk.Error != nil {
		return nil, false, fault.Wrap(fault.StreamFailed, p.name+": provider reported an error mid-stream", vendorError{message: chunk.Error.Message, code: fmt.Sprint(chunk.Error.Code)})
	}
	if chunk.ID != "" {
		p.id = chunk.ID
	}
	if chunk.Model != "" {
		p.model = chunk.Model
	}

	var events []provider.Event
	if len(chunk.Choices) > 0 {
		events = p.choice(chunk.Choices[0])
	}
	if chunk.Usage != nil {
		events = append(events, provider.Event{Type: provider.EventUsage, Usage: convertUsage(*chunk.Usage)})
	}
	return events, false, nil
}

func (p *streamParser) choice(choice chatChoice) []provider.Event {
	var events []provider.Event
	if choice.Delta.ReasoningContent != "" {
		events = append(events, provider.Event{Type: provider.EventReasoningDelta, Delta: choice.Delta.ReasoningContent})
	}
	if choice.Delta.Content != "" {
		events = append(events, provider.Event{Type: provider.EventTextDelta, Delta: choice.Delta.Content})
	}
	for position, delta := range choice.Delta.ToolCalls {
		events = append(events, p.toolCallDelta(position, delta)...)
	}
	if choice.FinishReason != "" {
		p.finish = choice.FinishReason
		events = append(events, p.completeCalls()...)
	}
	return events
}

func (p *streamParser) toolCallDelta(position int, delta chatToolCall) []provider.Event {
	index := position
	if delta.Index != nil {
		index = *delta.Index
	}
	var events []provider.Event
	call, exists := p.calls[index]
	if !exists {
		call = &pendingCall{id: delta.ID, name: p.names.decode(delta.Function.Name)}
		if call.id == "" {
			call.id = fmt.Sprintf("call_%d", index)
		}
		p.calls[index] = call
		p.order = append(p.order, index)
		events = append(events, provider.Event{
			Type:     provider.EventToolCallStarted,
			ToolCall: message.ToolCall{ID: call.id, Name: call.name},
		})
	}
	if fragment := delta.Function.Arguments; fragment != "" {
		call.arguments.WriteString(fragment)
		events = append(events, provider.Event{
			Type:     provider.EventToolCallDelta,
			ToolCall: message.ToolCall{ID: call.id},
			Delta:    fragment,
		})
	}
	return events
}

func (p *streamParser) completeCalls() []provider.Event {
	var events []provider.Event
	for _, index := range p.order {
		call := p.calls[index]
		if call.completed {
			continue
		}
		call.completed = true
		var arguments json.RawMessage
		if call.arguments.Len() > 0 {
			arguments = json.RawMessage(call.arguments.String())
		}
		events = append(events, provider.Event{
			Type:     provider.EventToolCallCompleted,
			ToolCall: message.ToolCall{ID: call.id, Name: call.name, Arguments: arguments},
		})
	}
	return events
}

func (p *streamParser) done() ([]provider.Event, error) {
	events := p.completeCalls()
	switch p.finish {
	case "":
		return nil, fault.New(fault.StreamFailed, p.name+": stream finished without a finish reason")
	case "insufficient_system_resource":
		return nil, fault.New(fault.ProviderUnavailable, p.name+": provider ran out of resources mid-generation")
	case "aborted":
		return nil, fault.New(fault.StreamFailed, p.name+": provider aborted the generation")
	}
	return append(events, provider.Event{
		Type:         provider.EventCompleted,
		FinishReason: convertFinishReason(p.finish),
		Model:        p.model,
		RequestID:    p.id,
	}), nil
}

func convertFinishReason(raw string) provider.FinishReason {
	switch raw {
	case "stop":
		return provider.FinishStop
	case "length":
		return provider.FinishLength
	case "tool_calls", "function_call":
		return provider.FinishToolCalls
	case "content_filter":
		return provider.FinishContentFilter
	}
	return provider.FinishOther
}

func convertUsage(u chatUsage) usage.Usage {
	out := usage.Usage{
		InputTokens:  count(u.PromptTokens),
		OutputTokens: count(u.CompletionTokens),
	}
	switch {
	case u.PromptCacheHitTokens != nil:
		out.CachedInputTokens = count(u.PromptCacheHitTokens)
	case u.PromptTokensDetails != nil:
		out.CachedInputTokens = count(u.PromptTokensDetails.CachedTokens)
	}
	if u.CompletionTokensDetails != nil {
		out.ReasoningTokens = count(u.CompletionTokensDetails.ReasoningTokens)
	}
	return out
}

func count(v *int64) usage.Count {
	if v == nil {
		return usage.Count{}
	}
	return usage.Known(*v)
}
