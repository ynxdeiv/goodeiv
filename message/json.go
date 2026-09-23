package message

import (
	"encoding/json"
	"fmt"

	"github.com/ynxdeiv/goodeiv/fault"
)

type jsonMessage struct {
	Role  Role       `json:"role"`
	Parts []jsonPart `json:"parts"`
}

type jsonPart struct {
	Type      partType        `json:"type"`
	Text      string          `json:"text,omitempty"`
	MediaType string          `json:"media_type,omitempty"`
	Name      string          `json:"name,omitempty"`
	Data      []byte          `json:"data,omitempty"`
	URL       string          `json:"url,omitempty"`
	ID        string          `json:"id,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Trust     *Trust          `json:"trust,omitempty"`
}

// MarshalJSON encodes m with a "type" discriminator on every part.
func (m Message) MarshalJSON() ([]byte, error) {
	out := jsonMessage{Role: m.Role, Parts: make([]jsonPart, 0, len(m.Parts))}
	for i, part := range m.Parts {
		encoded, err := encodePart(part)
		if err != nil {
			return nil, fmt.Errorf("message: part %d: %w", i, err)
		}
		out.Parts = append(out.Parts, encoded)
	}
	return json.Marshal(out)
}

// UnmarshalJSON decodes a message produced by MarshalJSON. Unknown part types are rejected
// with fault.InvalidInput.
func (m *Message) UnmarshalJSON(data []byte) error {
	var in jsonMessage
	if err := json.Unmarshal(data, &in); err != nil {
		return fault.Wrap(fault.InvalidInput, "message: malformed JSON", err)
	}
	parts := make([]Part, 0, len(in.Parts))
	for i, encoded := range in.Parts {
		part, err := decodePart(encoded)
		if err != nil {
			return fault.Wrap(fault.InvalidInput, fmt.Sprintf("message: part %d: %s", i, err), err)
		}
		parts = append(parts, part)
	}
	*m = Message{Role: in.Role, Parts: parts}
	return nil
}

func encodePart(part Part) (jsonPart, error) {
	switch p := part.(type) {
	case Text:
		return jsonPart{Type: partText, Text: p.Text}, nil
	case Reasoning:
		return jsonPart{Type: partReasoning, Text: p.Text}, nil
	case Image:
		return jsonPart{Type: partImage, MediaType: p.MediaType, Data: p.Data, URL: p.URL}, nil
	case File:
		return jsonPart{Type: partFile, MediaType: p.MediaType, Name: p.Name, Data: p.Data, URL: p.URL}, nil
	case ToolCall:
		return jsonPart{Type: partToolCall, ID: p.ID, Name: p.Name, Arguments: p.Arguments}, nil
	case ToolResult:
		trust := p.Trust
		return jsonPart{Type: partToolResult, CallID: p.CallID, Name: p.Name, Text: p.Text, IsError: p.IsError, Trust: &trust}, nil
	}
	return jsonPart{}, fmt.Errorf("unsupported part %T", part)
}

func decodePart(p jsonPart) (Part, error) {
	switch p.Type {
	case partText:
		return Text{Text: p.Text}, nil
	case partReasoning:
		return Reasoning{Text: p.Text}, nil
	case partImage:
		return Image{MediaType: p.MediaType, Data: p.Data, URL: p.URL}, nil
	case partFile:
		return File{MediaType: p.MediaType, Name: p.Name, Data: p.Data, URL: p.URL}, nil
	case partToolCall:
		return ToolCall{ID: p.ID, Name: p.Name, Arguments: p.Arguments}, nil
	case partToolResult:
		result := ToolResult{CallID: p.CallID, Name: p.Name, Text: p.Text, IsError: p.IsError}
		if p.Trust != nil {
			result.Trust = *p.Trust
		}
		return result, nil
	}
	return nil, fmt.Errorf("unknown part type %q", p.Type)
}

// MarshalText encodes t as its lowercase name.
func (t Trust) MarshalText() ([]byte, error) {
	name, ok := trustNames[t]
	if !ok {
		return nil, fmt.Errorf("message: unknown trust level %d", uint8(t))
	}
	return []byte(name), nil
}

// UnmarshalText decodes a lowercase trust name.
func (t *Trust) UnmarshalText(text []byte) error {
	for level, name := range trustNames {
		if name == string(text) {
			*t = level
			return nil
		}
	}
	return fmt.Errorf("message: unknown trust level %q", text)
}
