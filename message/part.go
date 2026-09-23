package message

import (
	"encoding/json"
	"fmt"
)

// Part is a piece of message content. The set of implementations is closed:
// Text, Image, File, ToolCall and ToolResult.
type Part interface {
	partType() partType
}

type partType string

const (
	partText       partType = "text"
	partImage      partType = "image"
	partFile       partType = "file"
	partToolCall   partType = "tool_call"
	partToolResult partType = "tool_result"
)

// Text is plain text content.
type Text struct {
	// Text is the content.
	Text string
}

// Image is image content provided either inline through Data or by URL.
type Image struct {
	// MediaType is the IANA media type, such as "image/png".
	MediaType string
	// Data holds the raw bytes when the image is inline.
	Data []byte
	// URL locates the image when it is not inline.
	URL string
}

// File is document content, such as a PDF, provided either inline through Data or by URL.
type File struct {
	// MediaType is the IANA media type, such as "application/pdf".
	MediaType string
	// Name is an optional file name shown to the model.
	Name string
	// Data holds the raw bytes when the file is inline.
	Data []byte
	// URL locates the file when it is not inline.
	URL string
}

// ToolCall is a request from the model to invoke a tool.
type ToolCall struct {
	// ID correlates the call with its ToolResult.
	ID string
	// Name is the registered tool name.
	Name string
	// Arguments is the JSON object produced by the model. Empty means no arguments.
	Arguments json.RawMessage
}

// ToolResult is the outcome of a tool call, sent back to the model.
type ToolResult struct {
	// CallID is the ID of the ToolCall this result answers.
	CallID string
	// Name is the tool name. Some providers require it alongside CallID.
	Name string
	// Text is the content shown to the model.
	Text string
	// IsError marks the result as a tool failure the model should react to.
	IsError bool
	// Trust states whether Text may be treated as instructions. The zero value is Untrusted.
	Trust Trust
}

func (Text) partType() partType       { return partText }
func (Image) partType() partType      { return partImage }
func (File) partType() partType       { return partFile }
func (ToolCall) partType() partType   { return partToolCall }
func (ToolResult) partType() partType { return partToolResult }

// Trust states whether content comes from a trusted source. Content from emails, documents,
// web pages or external APIs must stay Untrusted so policies can guard against prompt injection.
type Trust uint8

// Trust levels. The zero value is Untrusted.
const (
	Untrusted Trust = iota
	Trusted
)

var trustNames = map[Trust]string{
	Untrusted: "untrusted",
	Trusted:   "trusted",
}

// String returns the lowercase name of t.
func (t Trust) String() string {
	if name, ok := trustNames[t]; ok {
		return name
	}
	return fmt.Sprintf("trust(%d)", uint8(t))
}
