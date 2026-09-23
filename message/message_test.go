package message_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
)

func TestConstructors(t *testing.T) {
	tests := []struct {
		name string
		msg  message.Message
		role message.Role
		text string
	}{
		{"system", message.System("be brief"), message.RoleSystem, "be brief"},
		{"user", message.User("hi"), message.RoleUser, "hi"},
		{"assistant", message.Assistant("hello"), message.RoleAssistant, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msg.Role != tt.role {
				t.Fatalf("role = %q, want %q", tt.msg.Role, tt.role)
			}
			if got := tt.msg.Text(); got != tt.text {
				t.Fatalf("Text() = %q, want %q", got, tt.text)
			}
			if err := tt.msg.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestTextJoinsOnlyTextParts(t *testing.T) {
	msg := message.New(message.RoleAssistant,
		message.Text{Text: "Checking "},
		message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{}`)},
		message.Text{Text: "now."},
	)

	if got, want := msg.Text(), "Checking now."; got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestToolCallsAndResults(t *testing.T) {
	call := message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{"city":"Lisbon"}`)}
	assistant := message.New(message.RoleAssistant, message.Text{Text: "one sec"}, call)

	if got := assistant.ToolCalls(); !reflect.DeepEqual(got, []message.ToolCall{call}) {
		t.Fatalf("ToolCalls() = %+v", got)
	}

	result := message.ToolResult{CallID: "c1", Name: "weather.current", Text: "22°C"}
	tool := message.Tool(result)

	if tool.Role != message.RoleTool {
		t.Fatalf("role = %q, want tool", tool.Role)
	}
	if got := tool.ToolResults(); !reflect.DeepEqual(got, []message.ToolResult{result}) {
		t.Fatalf("ToolResults() = %+v", got)
	}
}

func TestToolResultsAreUntrustedByDefault(t *testing.T) {
	var result message.ToolResult
	if result.Trust != message.Untrusted {
		t.Fatal("the zero ToolResult must be untrusted")
	}
}

func TestValidate(t *testing.T) {
	validCall := message.ToolCall{ID: "c1", Name: "gmail.search", Arguments: json.RawMessage(`{"q":"x"}`)}

	tests := []struct {
		name    string
		msg     message.Message
		wantErr bool
	}{
		{"user text", message.User("hi"), false},
		{"user image by url", message.New(message.RoleUser, message.Image{MediaType: "image/png", URL: "https://x/y.png"}), false},
		{"user file by data", message.New(message.RoleUser, message.File{MediaType: "application/pdf", Data: []byte("%PDF")}), false},
		{"assistant tool call", message.New(message.RoleAssistant, validCall), false},
		{"assistant tool call without arguments", message.New(message.RoleAssistant, message.ToolCall{ID: "c1", Name: "x"}), false},
		{"tool result", message.Tool(message.ToolResult{CallID: "c1", Text: "ok"}), false},

		{"unknown role", message.New(message.Role("robot"), message.Text{Text: "x"}), true},
		{"no parts", message.New(message.RoleUser), true},
		{"empty text", message.User(""), true},
		{"system with image", message.New(message.RoleSystem, message.Image{MediaType: "image/png", URL: "u"}), true},
		{"user with tool call", message.New(message.RoleUser, validCall), true},
		{"assistant with tool result", message.New(message.RoleAssistant, message.ToolResult{CallID: "c1"}), true},
		{"tool with text", message.New(message.RoleTool, message.Text{Text: "x"}), true},
		{"tool call without id", message.New(message.RoleAssistant, message.ToolCall{Name: "x"}), true},
		{"tool call without name", message.New(message.RoleAssistant, message.ToolCall{ID: "c1"}), true},
		{"tool call with invalid json", message.New(message.RoleAssistant, message.ToolCall{ID: "c1", Name: "x", Arguments: json.RawMessage(`{`)}), true},
		{"tool call with null arguments", message.New(message.RoleAssistant, message.ToolCall{ID: "c1", Name: "x", Arguments: json.RawMessage(`null`)}), true},
		{"tool call with non-object json", message.New(message.RoleAssistant, message.ToolCall{ID: "c1", Name: "x", Arguments: json.RawMessage(`[1]`)}), true},
		{"tool result without call id", message.Tool(message.ToolResult{Text: "x"}), true},
		{"image without source", message.New(message.RoleUser, message.Image{MediaType: "image/png"}), true},
		{"image with both sources", message.New(message.RoleUser, message.Image{MediaType: "image/png", URL: "u", Data: []byte{1}}), true},
		{"image without media type", message.New(message.RoleUser, message.Image{URL: "u"}), true},
		{"file without source", message.New(message.RoleUser, message.File{MediaType: "text/plain"}), true},
		{"nil part", message.New(message.RoleUser, nil), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if tt.wantErr != (err != nil) {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, fault.InvalidInput) {
				t.Fatalf("expected fault.InvalidInput, got %v", err)
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	conversation := []message.Message{
		message.System("be brief"),
		message.New(message.RoleUser,
			message.Text{Text: "what is in this image?"},
			message.Image{MediaType: "image/png", Data: []byte{0x89, 0x50}},
			message.File{MediaType: "application/pdf", Name: "a.pdf", URL: "https://x/a.pdf"},
		),
		message.New(message.RoleAssistant,
			message.Text{Text: "checking"},
			message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{"city":"Lisbon"}`)},
		),
		message.Tool(message.ToolResult{CallID: "c1", Name: "weather.current", Text: "22°C", IsError: true, Trust: message.Trusted}),
		message.Tool(message.ToolResult{CallID: "c2", Text: "email body"}),
	}

	data, err := json.Marshal(conversation)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded []message.Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, conversation) {
		t.Fatalf("round trip mismatch\n got: %+v\nwant: %+v", decoded, conversation)
	}
}

func TestJSONRejectsUnknownPartType(t *testing.T) {
	var msg message.Message
	err := json.Unmarshal([]byte(`{"role":"user","parts":[{"type":"hologram"}]}`), &msg)
	if err == nil {
		t.Fatal("expected an error for an unknown part type")
	}
	if !errors.Is(err, fault.InvalidInput) {
		t.Fatalf("expected fault.InvalidInput, got %v", err)
	}
}

func TestJSONRejectsUnknownTrust(t *testing.T) {
	var msg message.Message
	err := json.Unmarshal([]byte(`{"role":"tool","parts":[{"type":"tool_result","call_id":"c1","trust":"maybe"}]}`), &msg)
	if err == nil {
		t.Fatal("expected an error for an unknown trust level")
	}
}

func ExampleNew() {
	msg := message.New(message.RoleUser,
		message.Text{Text: "Summarize this contract."},
		message.File{MediaType: "application/pdf", Name: "contract.pdf", URL: "https://example.com/contract.pdf"},
	)

	fmt.Println(msg.Role)
	fmt.Println(msg.Text())
	fmt.Println(msg.Validate() == nil)
	// Output:
	// user
	// Summarize this contract.
	// true
}
