// Package providertest provides a scripted, network-free provider.Provider for tests.
//
// Each call to Generate or Stream consumes the next scripted Turn and records the request, so
// tests can drive an agent loop deterministically and assert on what the model was sent.
package providertest

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"sync"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/usage"
)

// Turn is one scripted model response.
type Turn struct {
	text  string
	calls []message.ToolCall
	usage usage.Usage
	err   error
}

// Reply returns a turn that answers with text and finishes with provider.FinishStop.
func Reply(text string) Turn {
	return Turn{text: text}
}

// CallTools returns a turn that requests the given tool calls.
func CallTools(calls ...message.ToolCall) Turn {
	return Turn{calls: calls}
}

// Fail returns a turn that fails with err before producing any output.
func Fail(err error) Turn {
	return Turn{err: err}
}

// Interrupt returns a turn that streams text and then fails with err, as a dropped connection would.
func Interrupt(text string, err error) Turn {
	return Turn{text: text, err: err}
}

// WithUsage returns a copy of t that reports u.
func (t Turn) WithUsage(u usage.Usage) Turn {
	t.usage = u
	return t
}

// Provider is a scripted provider.Provider. It is safe for concurrent use.
type Provider struct {
	mu           sync.Mutex
	turns        []Turn
	requests     []provider.Request
	capabilities provider.Capabilities
}

// New returns a provider that plays turns in order and supports every capability.
func New(turns ...Turn) *Provider {
	return &Provider{
		turns: turns,
		capabilities: provider.Capabilities{
			Tools:             true,
			ParallelToolCalls: true,
			ForcedToolChoice:  true,
			Vision:            true,
			Files:             true,
			StructuredOutput:  true,
			Reasoning:         true,
			PromptCaching:     true,
		},
	}
}

// WithCapabilities replaces the capabilities reported for every model and returns p.
func (p *Provider) WithCapabilities(c provider.Capabilities) *Provider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.capabilities = c
	return p
}

// Capabilities returns the configured capabilities, whatever the model.
func (p *Provider) Capabilities(string) provider.Capabilities {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.capabilities
}

// Requests returns every valid request received so far, in order.
func (p *Provider) Requests() []provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]provider.Request(nil), p.requests...)
}

// Remaining returns how many scripted turns have not been played yet.
func (p *Provider) Remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.turns)
}

// Generate plays the next turn and returns the collected response.
func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	return provider.Collect(p.Stream(ctx, req))
}

// Stream validates req, records it and plays the next turn as stream events. Invalid requests
// do not consume a turn. Cancellation of ctx is reported before every event.
func (p *Provider) Stream(ctx context.Context, req provider.Request) iter.Seq2[provider.Event, error] {
	return func(yield func(provider.Event, error) bool) {
		if err := req.Validate(); err != nil {
			yield(provider.Event{}, err)
			return
		}
		turn, sequence, err := p.next(req)
		if err != nil {
			yield(provider.Event{}, err)
			return
		}
		for _, ev := range turn.events(req.Model, sequence) {
			if err := ctx.Err(); err != nil {
				yield(provider.Event{}, err)
				return
			}
			if !yield(ev, nil) {
				return
			}
		}
		if turn.err != nil {
			yield(provider.Event{}, turn.err)
		}
	}
}

func (p *Provider) next(req provider.Request) (Turn, int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if len(p.turns) == 0 {
		return Turn{}, 0, fault.New(fault.Internal, "providertest: no scripted turn left")
	}
	turn := p.turns[0]
	p.turns = p.turns[1:]
	return turn, len(p.requests), nil
}

func (t Turn) events(model string, sequence int) []provider.Event {
	var evs []provider.Event
	for _, chunk := range chunks(t.text) {
		evs = append(evs, provider.Event{Type: provider.EventTextDelta, Delta: chunk})
	}
	if t.err != nil {
		return evs
	}
	for _, call := range t.calls {
		evs = append(evs,
			provider.Event{Type: provider.EventToolCallStarted, ToolCall: message.ToolCall{ID: call.ID, Name: call.Name}},
			provider.Event{Type: provider.EventToolCallDelta, ToolCall: message.ToolCall{ID: call.ID}, Delta: string(call.Arguments)},
			provider.Event{Type: provider.EventToolCallCompleted, ToolCall: call},
		)
	}
	if t.usage != (usage.Usage{}) {
		evs = append(evs, provider.Event{Type: provider.EventUsage, Usage: t.usage})
	}
	finish := provider.FinishStop
	if len(t.calls) > 0 {
		finish = provider.FinishToolCalls
	}
	return append(evs, provider.Event{
		Type:         provider.EventCompleted,
		FinishReason: finish,
		Model:        model,
		RequestID:    fmt.Sprintf("providertest-%d", sequence),
	})
}

func chunks(text string) []string {
	if text == "" {
		return nil
	}
	words := strings.SplitAfter(text, " ")
	out := words[:0]
	for _, word := range words {
		if word != "" {
			out = append(out, word)
		}
	}
	return out
}
