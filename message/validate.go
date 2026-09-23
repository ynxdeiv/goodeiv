package message

import (
	"encoding/json"
	"fmt"

	"github.com/ynxdeiv/goodeiv/fault"
)

var allowedParts = map[Role]map[partType]bool{
	RoleSystem:    {partText: true},
	RoleUser:      {partText: true, partImage: true, partFile: true},
	RoleAssistant: {partText: true, partToolCall: true},
	RoleTool:      {partToolResult: true},
}

// Validate reports whether m is well formed: a known role, at least one part, only part types
// allowed for that role, and every part internally consistent. Errors are fault.InvalidInput.
func (m Message) Validate() error {
	if !m.Role.Valid() {
		return invalid("unknown role %q", m.Role)
	}
	if len(m.Parts) == 0 {
		return invalid("%s message has no parts", m.Role)
	}
	for i, part := range m.Parts {
		if part == nil {
			return invalid("%s message part %d is nil", m.Role, i)
		}
		if !allowedParts[m.Role][part.partType()] {
			return invalid("%s message cannot contain %s part", m.Role, part.partType())
		}
		if err := validatePart(part); err != nil {
			return invalid("%s message part %d: %s", m.Role, i, err)
		}
	}
	return nil
}

func validatePart(part Part) error {
	switch p := part.(type) {
	case Text:
		if p.Text == "" {
			return fmt.Errorf("text is empty")
		}
	case Image:
		return validateSource(p.MediaType, p.Data, p.URL)
	case File:
		return validateSource(p.MediaType, p.Data, p.URL)
	case ToolCall:
		return validateToolCall(p)
	case ToolResult:
		if p.CallID == "" {
			return fmt.Errorf("tool result has no call id")
		}
	}
	return nil
}

func validateSource(mediaType string, data []byte, url string) error {
	if mediaType == "" {
		return fmt.Errorf("media type is empty")
	}
	if (len(data) == 0) == (url == "") {
		return fmt.Errorf("exactly one of data or url must be set")
	}
	return nil
}

func validateToolCall(call ToolCall) error {
	if call.ID == "" {
		return fmt.Errorf("tool call has no id")
	}
	if call.Name == "" {
		return fmt.Errorf("tool call has no name")
	}
	if len(call.Arguments) == 0 {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(call.Arguments, &object); err != nil || object == nil {
		return fmt.Errorf("tool call %q arguments are not a JSON object", call.Name)
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fault.New(fault.InvalidInput, "message: "+fmt.Sprintf(format, args...))
}
