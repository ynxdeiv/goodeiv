// Package message defines the provider-agnostic conversation model: roles, messages and the
// typed parts they are made of.
package message

import "strings"

// Role identifies who authored a message.
type Role string

// Roles.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Valid reports whether r is one of the declared roles.
func (r Role) Valid() bool {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	}
	return false
}

// Message is one turn of a conversation.
type Message struct {
	// Role is the author of the message.
	Role Role
	// Parts is the ordered content of the message.
	Parts []Part
}

// New returns a message with the given role and parts.
func New(role Role, parts ...Part) Message {
	return Message{Role: role, Parts: parts}
}

// System returns a system message containing text.
func System(text string) Message {
	return New(RoleSystem, Text{Text: text})
}

// User returns a user message containing text.
func User(text string) Message {
	return New(RoleUser, Text{Text: text})
}

// Assistant returns an assistant message containing text.
func Assistant(text string) Message {
	return New(RoleAssistant, Text{Text: text})
}

// Tool returns a tool message carrying the given results.
func Tool(results ...ToolResult) Message {
	parts := make([]Part, len(results))
	for i, result := range results {
		parts[i] = result
	}
	return New(RoleTool, parts...)
}

// Text returns the concatenation of every Text part, ignoring other part types.
func (m Message) Text() string {
	var b strings.Builder
	for _, part := range m.Parts {
		if text, ok := part.(Text); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

// ToolCalls returns the tool calls in the message, in order.
func (m Message) ToolCalls() []ToolCall {
	return partsOf[ToolCall](m.Parts)
}

// ToolResults returns the tool results in the message, in order.
func (m Message) ToolResults() []ToolResult {
	return partsOf[ToolResult](m.Parts)
}

func partsOf[T Part](parts []Part) []T {
	var out []T
	for _, part := range parts {
		if typed, ok := part.(T); ok {
			out = append(out, typed)
		}
	}
	return out
}
