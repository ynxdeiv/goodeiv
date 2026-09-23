package provider_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
)

func validRequest() provider.Request {
	return provider.Request{
		Model:    "model-x",
		Messages: []message.Message{message.System("be brief"), message.User("hi")},
		Tools: []provider.ToolSpec{
			{Name: "weather.current", Description: "Current weather.", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}
}

func TestRequestValidate(t *testing.T) {
	temperature := func(v float64) *float64 { return &v }
	call := message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{}`)}

	tests := []struct {
		name    string
		mutate  func(*provider.Request)
		wantErr bool
	}{
		{"valid", func(*provider.Request) {}, false},
		{"tool call answered by a later result", func(r *provider.Request) {
			r.Messages = append(r.Messages,
				message.New(message.RoleAssistant, call),
				message.Tool(message.ToolResult{CallID: "c1", Text: "sunny"}),
			)
		}, false},
		{"named tool choice", func(r *provider.Request) {
			r.ToolChoice = provider.ToolChoice{Mode: provider.ToolChoiceTool, Name: "weather.current"}
		}, false},
		{"zero temperature", func(r *provider.Request) { r.Temperature = temperature(0) }, false},
		{"no tools and no choice", func(r *provider.Request) { r.Tools = nil }, false},

		{"missing model", func(r *provider.Request) { r.Model = "" }, true},
		{"no messages", func(r *provider.Request) { r.Messages = nil }, true},
		{"only system messages", func(r *provider.Request) { r.Messages = []message.Message{message.System("x")} }, true},
		{"system after user", func(r *provider.Request) { r.Messages = append(r.Messages, message.System("late")) }, true},
		{"invalid message", func(r *provider.Request) { r.Messages = append(r.Messages, message.User("")) }, true},
		{"orphan tool result", func(r *provider.Request) {
			r.Messages = append(r.Messages, message.Tool(message.ToolResult{CallID: "missing", Text: "x"}))
		}, true},
		{"duplicate tool call id", func(r *provider.Request) {
			r.Messages = append(r.Messages, message.New(message.RoleAssistant, call, call))
		}, true},
		{"tool without name", func(r *provider.Request) { r.Tools[0].Name = "" }, true},
		{"tool with invalid name", func(r *provider.Request) { r.Tools[0].Name = "has space" }, true},
		{"duplicate tool", func(r *provider.Request) { r.Tools = append(r.Tools, r.Tools[0]) }, true},
		{"tool schema not an object", func(r *provider.Request) { r.Tools[0].InputSchema = json.RawMessage(`[]`) }, true},
		{"unknown tool choice mode", func(r *provider.Request) { r.ToolChoice = provider.ToolChoice{Mode: "sometimes"} }, true},
		{"named choice without name", func(r *provider.Request) { r.ToolChoice = provider.ToolChoice{Mode: provider.ToolChoiceTool} }, true},
		{"named choice for unknown tool", func(r *provider.Request) {
			r.ToolChoice = provider.ToolChoice{Mode: provider.ToolChoiceTool, Name: "nope"}
		}, true},
		{"required choice without tools", func(r *provider.Request) {
			r.Tools = nil
			r.ToolChoice = provider.ToolChoice{Mode: provider.ToolChoiceRequired}
		}, true},
		{"name set outside tool mode", func(r *provider.Request) {
			r.ToolChoice = provider.ToolChoice{Mode: provider.ToolChoiceAuto, Name: "weather.current"}
		}, true},
		{"negative max output tokens", func(r *provider.Request) { r.MaxOutputTokens = -1 }, true},
		{"negative temperature", func(r *provider.Request) { r.Temperature = temperature(-0.1) }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRequest()
			tt.mutate(&req)

			err := req.Validate()
			if tt.wantErr != (err != nil) {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, fault.InvalidInput) {
				t.Fatalf("expected fault.InvalidInput, got %v", err)
			}
		})
	}
}

func forced(choice provider.ToolChoice) provider.Request {
	req := validRequest()
	req.ToolChoice = choice
	return req
}

func TestCapabilitiesSupports(t *testing.T) {
	image := message.New(message.RoleUser, message.Image{MediaType: "image/png", URL: "https://x/y.png"})
	file := message.New(message.RoleUser, message.File{MediaType: "application/pdf", URL: "https://x/y.pdf"})

	tests := []struct {
		name    string
		caps    provider.Capabilities
		req     provider.Request
		wantErr bool
	}{
		{"text only needs nothing", provider.Capabilities{}, provider.Request{Messages: []message.Message{message.User("hi")}}, false},
		{"tools need tool support", provider.Capabilities{}, validRequest(), true},
		{"tools supported", provider.Capabilities{Tools: true}, validRequest(), false},
		{"images need vision", provider.Capabilities{}, provider.Request{Messages: []message.Message{image}}, true},
		{"images supported", provider.Capabilities{Vision: true}, provider.Request{Messages: []message.Message{image}}, false},
		{"files need file support", provider.Capabilities{Vision: true}, provider.Request{Messages: []message.Message{file}}, true},
		{"files supported", provider.Capabilities{Files: true}, provider.Request{Messages: []message.Message{file}}, false},
		{"required choice needs forced tool choice", provider.Capabilities{Tools: true}, forced(provider.ToolChoice{Mode: provider.ToolChoiceRequired}), true},
		{"named choice needs forced tool choice", provider.Capabilities{Tools: true}, forced(provider.ToolChoice{Mode: provider.ToolChoiceTool, Name: "weather.current"}), true},
		{"forced tool choice supported", provider.Capabilities{Tools: true, ForcedToolChoice: true}, forced(provider.ToolChoice{Mode: provider.ToolChoiceRequired}), false},
		{"none choice needs nothing extra", provider.Capabilities{Tools: true}, forced(provider.ToolChoice{Mode: provider.ToolChoiceNone}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.caps.Supports(tt.req)
			if tt.wantErr != (err != nil) {
				t.Fatalf("Supports() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, fault.InvalidInput) {
				t.Fatalf("expected fault.InvalidInput, got %v", err)
			}
		})
	}
}
