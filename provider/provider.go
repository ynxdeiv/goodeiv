// Package provider defines the contract between the runtime and LLM vendors, plus the
// vendor-neutral request, response and stream event types adapters translate to and from.
//
// Adapters live in subpackages and keep every vendor type private. Most adapters only need to
// implement Stream and can implement Generate as Collect(p.Stream(ctx, req)).
package provider

import (
	"context"
	"iter"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
)

// Provider generates model output from a normalized Request.
type Provider interface {
	// Generate returns the complete response for req.
	Generate(ctx context.Context, req Request) (Response, error)
	// Stream yields normalized events for req. The consumer may stop at any time by breaking
	// out of the loop; the adapter must then release every resource it holds. A failure is
	// yielded as a non-nil error and ends the stream.
	Stream(ctx context.Context, req Request) iter.Seq2[Event, error]
	// Capabilities reports what the given model really supports.
	Capabilities(model string) Capabilities
}

// Capabilities declares the features a model supports. Adapters must never claim a feature
// they cannot deliver.
type Capabilities struct {
	// Tools reports support for tool definitions and tool calls.
	Tools bool
	// ParallelToolCalls reports whether the model may return several tool calls in one turn.
	ParallelToolCalls bool
	// Vision reports support for image inputs.
	Vision bool
	// Files reports support for document inputs such as PDFs.
	Files bool
	// StructuredOutput reports native support for schema-constrained output.
	StructuredOutput bool
	// Reasoning reports whether the model exposes reasoning output or reasoning token usage.
	Reasoning bool
	// PromptCaching reports support for provider-side prompt caching.
	PromptCaching bool
}

// Supports returns a fault.InvalidInput error if req needs a feature c lacks.
func (c Capabilities) Supports(req Request) error {
	if len(req.Tools) > 0 && !c.Tools {
		return unsupported("tools")
	}
	for _, msg := range req.Messages {
		for _, part := range msg.Parts {
			switch part.(type) {
			case message.Image:
				if !c.Vision {
					return unsupported("image inputs")
				}
			case message.File:
				if !c.Files {
					return unsupported("file inputs")
				}
			}
		}
	}
	return nil
}

func unsupported(feature string) error {
	return fault.New(fault.InvalidInput, "provider: model does not support "+feature)
}
