//go:build live

package deepseek_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/provider/deepseek"
)

func liveProvider(t *testing.T) (*deepseek.Provider, string) {
	t.Helper()
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}
	model := os.Getenv("DEEPSEEK_MODEL")
	if model == "" {
		t.Skip("DEEPSEEK_MODEL is not set; run TestLiveListModels to see the available names")
	}
	p, err := deepseek.New(deepseek.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	return p, model
}

func TestLiveListModels(t *testing.T) {
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deepseek.DefaultBaseURL+"/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /models: HTTP %d", resp.StatusCode)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	for _, model := range list.Data {
		t.Logf("model: %s", model.ID)
	}
}

func TestLiveStreamText(t *testing.T) {
	p, model := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := p.Generate(ctx, provider.Request{
		Model:    model,
		Messages: []message.Message{message.User("Reply with exactly the word: pong")},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	input, _ := resp.Usage.InputTokens.Value()
	output, _ := resp.Usage.OutputTokens.Value()
	t.Logf("text=%q finish=%s model=%s reasoning_chars=%d input=%d output=%d",
		resp.Message.Text(), resp.FinishReason, resp.Model, len(resp.Message.Reasoning()), input, output)
	if resp.Message.Text() == "" {
		t.Fatal("empty answer")
	}
}

func TestLiveToolRoundTrip(t *testing.T) {
	p, model := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tools := []provider.ToolSpec{{
		Name:        "weather.current",
		Description: "Returns the current weather for a city.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
	}}
	conversation := []message.Message{message.User("What is the weather in Lisbon right now? Use the tool.")}

	first, err := p.Generate(ctx, provider.Request{Model: model, Messages: conversation, Tools: tools})
	if err != nil {
		t.Fatalf("first turn: %v", err)
	}
	calls := first.Message.ToolCalls()
	if len(calls) == 0 {
		t.Fatalf("expected a tool call, got text %q", first.Message.Text())
	}
	t.Logf("tool call: %s %s (reasoning_chars=%d)", calls[0].Name, calls[0].Arguments, len(first.Message.Reasoning()))

	results := make([]message.ToolResult, len(calls))
	for i, call := range calls {
		results[i] = message.ToolResult{CallID: call.ID, Name: call.Name, Text: `{"temperature_c":22,"sky":"sunny"}`}
	}
	conversation = append(conversation, first.Message, message.Tool(results...))

	second, err := p.Generate(ctx, provider.Request{Model: model, Messages: conversation, Tools: tools})
	if err != nil {
		t.Fatalf("second turn (reasoning echo): %v", err)
	}
	t.Logf("answer: %q finish=%s", second.Message.Text(), second.FinishReason)
	if second.Message.Text() == "" {
		t.Fatal("empty final answer")
	}
}
