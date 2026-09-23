package provider

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
)

// Request is a vendor-neutral model call.
type Request struct {
	// Model is the provider-specific model name.
	Model string
	// Messages is the conversation. System messages may only appear before all other messages.
	Messages []message.Message
	// Tools are the tools the model may call.
	Tools []ToolSpec
	// ToolChoice constrains tool usage. The zero value lets the model decide.
	ToolChoice ToolChoice
	// MaxOutputTokens caps generated tokens. Zero uses the provider default.
	MaxOutputTokens int
	// Temperature controls sampling. Nil uses the provider default.
	Temperature *float64
}

// ToolSpec describes a tool to the model.
type ToolSpec struct {
	// Name is unique within a request and matches [A-Za-z0-9_.-]{1,64}.
	Name string
	// Description tells the model when and how to use the tool.
	Description string
	// InputSchema is a JSON Schema object for the arguments. Empty means no arguments.
	InputSchema json.RawMessage
}

// ToolChoiceMode selects how the model may use tools.
type ToolChoiceMode string

// Tool choice modes. The zero value is ToolChoiceAuto.
const (
	ToolChoiceAuto     ToolChoiceMode = ""
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceTool     ToolChoiceMode = "tool"
)

// ToolChoice constrains tool usage for one request.
type ToolChoice struct {
	// Mode selects the behavior.
	Mode ToolChoiceMode
	// Name is the tool the model must call. It is only valid with ToolChoiceTool.
	Name string
}

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// Validate reports whether req is well formed before any tokens are spent. It checks the model,
// message ordering and content, that every tool result answers an earlier tool call, tool
// definitions and the tool choice. Errors are fault.InvalidInput.
func (r Request) Validate() error {
	if r.Model == "" {
		return invalid("model is empty")
	}
	if err := validateMessages(r.Messages); err != nil {
		return err
	}
	tools, err := validateTools(r.Tools)
	if err != nil {
		return err
	}
	if err := validateToolChoice(r.ToolChoice, tools); err != nil {
		return err
	}
	if r.MaxOutputTokens < 0 {
		return invalid("max output tokens is negative")
	}
	if r.Temperature != nil && *r.Temperature < 0 {
		return invalid("temperature is negative")
	}
	return nil
}

func validateMessages(messages []message.Message) error {
	if len(messages) == 0 {
		return invalid("request has no messages")
	}
	calls := map[string]bool{}
	conversationStarted := false
	for i, msg := range messages {
		if err := msg.Validate(); err != nil {
			return fault.Wrap(fault.InvalidInput, fmt.Sprintf("provider: message %d is invalid", i), err)
		}
		if msg.Role == message.RoleSystem {
			if conversationStarted {
				return invalid("message %d: system messages must precede the conversation", i)
			}
			continue
		}
		conversationStarted = true
		for _, call := range msg.ToolCalls() {
			if calls[call.ID] {
				return invalid("message %d: duplicate tool call id %q", i, call.ID)
			}
			calls[call.ID] = true
		}
		for _, result := range msg.ToolResults() {
			if !calls[result.CallID] {
				return invalid("message %d: tool result %q answers no earlier tool call", i, result.CallID)
			}
		}
	}
	if !conversationStarted {
		return invalid("request has only system messages")
	}
	return nil
}

func validateTools(specs []ToolSpec) (map[string]bool, error) {
	names := make(map[string]bool, len(specs))
	for i, spec := range specs {
		if !toolNamePattern.MatchString(spec.Name) {
			return nil, invalid("tool %d: invalid name %q", i, spec.Name)
		}
		if names[spec.Name] {
			return nil, invalid("tool %d: duplicate name %q", i, spec.Name)
		}
		names[spec.Name] = true
		if len(spec.InputSchema) > 0 {
			var schema map[string]json.RawMessage
			if err := json.Unmarshal(spec.InputSchema, &schema); err != nil || schema == nil {
				return nil, invalid("tool %q: input schema is not a JSON object", spec.Name)
			}
		}
	}
	return names, nil
}

func validateToolChoice(choice ToolChoice, tools map[string]bool) error {
	switch choice.Mode {
	case ToolChoiceAuto, ToolChoiceNone, ToolChoiceRequired:
		if choice.Name != "" {
			return invalid("tool choice name is only allowed with mode %q", ToolChoiceTool)
		}
		if choice.Mode == ToolChoiceRequired && len(tools) == 0 {
			return invalid("tool choice %q needs at least one tool", choice.Mode)
		}
	case ToolChoiceTool:
		if !tools[choice.Name] {
			return invalid("tool choice names unknown tool %q", choice.Name)
		}
	default:
		return invalid("unknown tool choice mode %q", choice.Mode)
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fault.New(fault.InvalidInput, "provider: "+fmt.Sprintf(format, args...))
}
