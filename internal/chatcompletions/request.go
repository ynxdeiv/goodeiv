package chatcompletions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
)

var emptyObjectSchema = json.RawMessage(`{"type":"object","properties":{}}`)

func (c *Client) buildRequest(req provider.Request, names toolNames) (chatRequest, error) {
	out := chatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}
	if c.cfg.StreamUsage {
		out.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	echoReasoning := c.cfg.EchoReasoning && len(req.Tools) > 0
	for _, msg := range req.Messages {
		converted, err := convertMessage(msg, names, echoReasoning)
		if err != nil {
			return chatRequest{}, err
		}
		out.Messages = append(out.Messages, converted...)
	}
	for _, spec := range req.Tools {
		schema := spec.InputSchema
		if len(schema) == 0 {
			schema = emptyObjectSchema
		}
		out.Tools = append(out.Tools, chatTool{
			Type:     "function",
			Function: chatFunction{Name: names.encode(spec.Name), Description: spec.Description, Parameters: schema},
		})
	}
	choice, err := convertToolChoice(req.ToolChoice, names)
	if err != nil {
		return chatRequest{}, err
	}
	out.ToolChoice = choice
	return out, nil
}

func convertMessage(msg message.Message, names toolNames, echoReasoning bool) ([]chatMessage, error) {
	switch msg.Role {
	case message.RoleSystem, message.RoleUser:
		if err := requireTextOnly(msg); err != nil {
			return nil, err
		}
		return []chatMessage{{Role: string(msg.Role), Content: ptr(msg.Text())}}, nil
	case message.RoleAssistant:
		return []chatMessage{convertAssistant(msg, names, echoReasoning)}, nil
	case message.RoleTool:
		results := msg.ToolResults()
		out := make([]chatMessage, 0, len(results))
		for _, result := range results {
			content := result.Text
			if result.IsError {
				content = "Error: " + content
			}
			out = append(out, chatMessage{Role: "tool", Content: &content, ToolCallID: result.CallID})
		}
		return out, nil
	}
	return nil, fault.New(fault.InvalidInput, fmt.Sprintf("chatcompletions: unsupported role %q", msg.Role))
}

func convertAssistant(msg message.Message, names toolNames, echoReasoning bool) chatMessage {
	out := chatMessage{Role: "assistant"}
	if text := msg.Text(); text != "" {
		out.Content = &text
	}
	for _, call := range msg.ToolCalls() {
		arguments := string(call.Arguments)
		if arguments == "" {
			arguments = "{}"
		}
		out.ToolCalls = append(out.ToolCalls, chatToolCall{
			ID:       call.ID,
			Type:     "function",
			Function: chatCallFunction{Name: names.encode(call.Name), Arguments: arguments},
		})
	}
	reasoning := msg.Reasoning()
	if reasoning != "" || (echoReasoning && len(out.ToolCalls) > 0) {
		out.ReasoningContent = &reasoning
	}
	return out
}

func requireTextOnly(msg message.Message) error {
	for _, part := range msg.Parts {
		if _, ok := part.(message.Text); !ok {
			return fault.New(fault.InvalidInput, fmt.Sprintf("chatcompletions: %s messages support text only", msg.Role))
		}
	}
	return nil
}

func convertToolChoice(choice provider.ToolChoice, names toolNames) (json.RawMessage, error) {
	switch choice.Mode {
	case provider.ToolChoiceAuto:
		return nil, nil
	case provider.ToolChoiceNone, provider.ToolChoiceRequired:
		return json.Marshal(string(choice.Mode))
	case provider.ToolChoiceTool:
		return json.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]string{"name": names.encode(choice.Name)},
		})
	}
	return nil, fault.New(fault.InvalidInput, fmt.Sprintf("chatcompletions: unsupported tool choice %q", choice.Mode))
}

type toolNames struct {
	decoded map[string]string
}

func newToolNames(req provider.Request) (toolNames, error) {
	names := toolNames{decoded: map[string]string{}}
	for _, spec := range req.Tools {
		encoded := encodeToolName(spec.Name)
		if existing, taken := names.decoded[encoded]; taken && existing != spec.Name {
			return toolNames{}, fault.New(fault.InvalidInput, fmt.Sprintf(
				"chatcompletions: tools %q and %q map to the same wire name %q", existing, spec.Name, encoded))
		}
		names.decoded[encoded] = spec.Name
	}
	return names, nil
}

func (n toolNames) encode(name string) string {
	return encodeToolName(name)
}

func (n toolNames) decode(wire string) string {
	if name, ok := n.decoded[wire]; ok {
		return name
	}
	return wire
}

func encodeToolName(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		}
		return '_'
	}, name)
}

func ptr[T any](v T) *T {
	return &v
}
