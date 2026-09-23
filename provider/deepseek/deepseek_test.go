package deepseek_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/provider/deepseek"
	"github.com/ynxdeiv/goodeiv/usage"
)

const testKey = "sk-test-secret"

type capture struct {
	body   map[string]any
	header http.Header
	calls  int
}

func sse(chunks ...string) string {
	var b strings.Builder
	for _, chunk := range chunks {
		b.WriteString("data: " + chunk + "\n\n")
	}
	return b.String()
}

func newServer(t *testing.T, status int, body string, headers map[string]string) (*deepseek.Provider, *capture) {
	t.Helper()
	captured := &capture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.calls++
		captured.header = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured.body)
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		if status == http.StatusOK {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	p, err := deepseek.New(deepseek.Config{APIKey: testKey, BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p, captured
}

func userRequest() provider.Request {
	return provider.Request{Model: "deepseek-flash", Messages: []message.Message{message.User("hi")}}
}

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := deepseek.New(deepseek.Config{})
	if !errors.Is(err, fault.InvalidInput) {
		t.Fatalf("New() = %v, want fault.InvalidInput", err)
	}
}

func TestGenerateTextWithReasoningAndUsage(t *testing.T) {
	p, captured := newServer(t, http.StatusOK, ": keep-alive\n\n"+sse(
		`{"id":"req-1","model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"The user "}}]}`,
		`{"id":"req-1","model":"deepseek-flash","choices":[{"index":0,"delta":{"reasoning_content":"greets."}}]}`,
		`{"id":"req-1","model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		`{"id":"req-1","model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}`,
		`{"id":"req-1","model":"deepseek-flash","choices":[],"usage":{"prompt_tokens":20,"completion_tokens":9,"total_tokens":29,"prompt_cache_hit_tokens":16,"prompt_cache_miss_tokens":4,"completion_tokens_details":{"reasoning_tokens":5}}}`,
		`[DONE]`,
	), nil)

	resp, err := p.Generate(context.Background(), userRequest())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if resp.Message.Text() != "Hello!" || resp.Message.Reasoning() != "The user greets." {
		t.Fatalf("message = %+v", resp.Message)
	}
	if resp.FinishReason != provider.FinishStop || resp.Model != "deepseek-flash" || resp.RequestID != "req-1" {
		t.Fatalf("metadata = %q %q %q", resp.FinishReason, resp.Model, resp.RequestID)
	}
	want := usage.Usage{
		InputTokens:       usage.Known(20),
		OutputTokens:      usage.Known(9),
		CachedInputTokens: usage.Known(16),
		ReasoningTokens:   usage.Known(5),
	}
	if resp.Usage != want {
		t.Fatalf("usage = %+v, want %+v", resp.Usage, want)
	}

	if got := captured.header.Get("Authorization"); got != "Bearer "+testKey {
		t.Fatalf("Authorization = %q", got)
	}
	if captured.body["stream"] != true {
		t.Fatal("request must stream")
	}
	if opts, _ := captured.body["stream_options"].(map[string]any); opts["include_usage"] != true {
		t.Fatalf("stream_options = %v", captured.body["stream_options"])
	}
}

func TestGenerateToolCalls(t *testing.T) {
	p, _ := newServer(t, http.StatusOK, sse(
		`{"id":"r","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather_current","arguments":""}}]}}]}`,
		`{"id":"r","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}`,
		`{"id":"r","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"gmail_search","arguments":"{}"}}]}}]}`,
		`{"id":"r","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Lisbon\"}"}}]}}]}`,
		`{"id":"r","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	), nil)

	req := userRequest()
	req.Tools = []provider.ToolSpec{
		{Name: "weather.current", Description: "Weather."},
		{Name: "gmail.search", Description: "Search email."},
	}

	resp, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	calls := resp.Message.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("tool calls = %+v", calls)
	}
	if calls[0].ID != "call_1" || calls[0].Name != "weather.current" || string(calls[0].Arguments) != `{"city":"Lisbon"}` {
		t.Fatalf("first call = %+v", calls[0])
	}
	if calls[1].ID != "call_2" || calls[1].Name != "gmail.search" || string(calls[1].Arguments) != `{}` {
		t.Fatalf("second call = %+v", calls[1])
	}
	if resp.FinishReason != provider.FinishToolCalls {
		t.Fatalf("finish = %q", resp.FinishReason)
	}
}

func TestStreamEmitsNormalizedToolEvents(t *testing.T) {
	p, _ := newServer(t, http.StatusOK, sse(
		`{"id":"r","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"weather_current","arguments":"{\"a\":"}}]}}]}`,
		`{"id":"r","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	), nil)
	req := userRequest()
	req.Tools = []provider.ToolSpec{{Name: "weather.current"}}

	var types []provider.EventType
	for ev, err := range p.Stream(context.Background(), req) {
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		types = append(types, ev.Type)
	}

	want := []provider.EventType{
		provider.EventToolCallStarted,
		provider.EventToolCallDelta,
		provider.EventToolCallDelta,
		provider.EventToolCallCompleted,
		provider.EventCompleted,
	}
	if fmt.Sprint(types) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
}

func TestRequestMapping(t *testing.T) {
	p, captured := newServer(t, http.StatusOK, sse(
		`{"id":"r","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
		`[DONE]`,
	), nil)
	temperature := 0.3
	call := message.ToolCall{ID: "call_1", Name: "weather.current", Arguments: json.RawMessage(`{"city":"Lisbon"}`)}
	req := provider.Request{
		Model: "deepseek-v4-pro",
		Messages: []message.Message{
			message.System("be brief"),
			message.User("weather?"),
			message.New(message.RoleAssistant,
				message.Reasoning{Text: "need the tool"},
				call,
				message.ToolCall{ID: "call_1b", Name: "weather.current"},
			),
			message.Tool(message.ToolResult{CallID: "call_1", Text: "sunny"}, message.ToolResult{CallID: "call_1b", Text: "x"}),
		},
		Tools:           []provider.ToolSpec{{Name: "weather.current", Description: "Weather.", InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)}},
		ToolChoice:      provider.ToolChoice{Mode: provider.ToolChoiceNone},
		MaxOutputTokens: 512,
		Temperature:     &temperature,
	}

	if _, err := p.Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	got, err := json.Marshal(captured.body)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"max_tokens":512,"messages":[` +
		`{"content":"be brief","role":"system"},` +
		`{"content":"weather?","role":"user"},` +
		`{"content":null,"reasoning_content":"need the tool","role":"assistant","tool_calls":[` +
		`{"function":{"arguments":"{\"city\":\"Lisbon\"}","name":"weather_current"},"id":"call_1","type":"function"},` +
		`{"function":{"arguments":"{}","name":"weather_current"},"id":"call_1b","type":"function"}]},` +
		`{"content":"sunny","role":"tool","tool_call_id":"call_1"},` +
		`{"content":"x","role":"tool","tool_call_id":"call_1b"}],` +
		`"model":"deepseek-v4-pro","stream":true,"stream_options":{"include_usage":true},"temperature":0.3,` +
		`"tool_choice":"none",` +
		`"tools":[{"function":{"description":"Weather.","name":"weather_current","parameters":{"properties":{"city":{"type":"string"}},"type":"object"}},"type":"function"}]}`
	if string(got) != want {
		t.Fatalf("request body\n got: %s\nwant: %s", got, want)
	}
}

func TestToolMessagesEchoReasoningWhenToolsArePresent(t *testing.T) {
	p, captured := newServer(t, http.StatusOK, sse(`{"id":"r","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`), nil)
	call := message.ToolCall{ID: "c1", Name: "weather.current"}
	req := provider.Request{
		Model: "deepseek-flash",
		Messages: []message.Message{
			message.User("weather?"),
			message.New(message.RoleAssistant, call),
			message.Tool(message.ToolResult{CallID: "c1", Text: "failed", IsError: true}),
		},
		Tools: []provider.ToolSpec{{Name: "weather.current"}},
	}

	if _, err := p.Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	messages := captured.body["messages"].([]any)
	assistant := messages[1].(map[string]any)
	if reasoning, ok := assistant["reasoning_content"]; !ok || reasoning != "" {
		t.Fatalf("assistant tool-call message must carry reasoning_content, got %v", assistant)
	}
	tool := messages[2].(map[string]any)
	if !strings.Contains(tool["content"].(string), "failed") || !strings.HasPrefix(tool["content"].(string), "Error:") {
		t.Fatalf("error tool results must be flagged to the model, got %q", tool["content"])
	}
	if schema := captured.body["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["parameters"]; schema == nil {
		t.Fatal("tools without a schema must still send an empty object schema")
	}
}

func TestToolNameCollisionIsRejected(t *testing.T) {
	p, captured := newServer(t, http.StatusOK, "", nil)
	req := userRequest()
	req.Tools = []provider.ToolSpec{{Name: "gmail.search"}, {Name: "gmail_search"}}

	_, err := p.Generate(context.Background(), req)
	if !errors.Is(err, fault.InvalidInput) {
		t.Fatalf("Generate() = %v, want fault.InvalidInput", err)
	}
	if captured.calls != 0 {
		t.Fatal("no HTTP call may happen for an invalid request")
	}
}

func TestInvalidOrUnsupportedRequestsNeverReachTheNetwork(t *testing.T) {
	p, captured := newServer(t, http.StatusOK, "", nil)
	image := provider.Request{
		Model:    "deepseek-flash",
		Messages: []message.Message{message.New(message.RoleUser, message.Image{MediaType: "image/png", URL: "https://x/y.png"})},
	}

	forcedChoice := userRequest()
	forcedChoice.Tools = []provider.ToolSpec{{Name: "weather.current"}}
	forcedChoice.ToolChoice = provider.ToolChoice{Mode: provider.ToolChoiceRequired}

	for name, req := range map[string]provider.Request{"invalid": {}, "image input": image, "forced tool choice": forcedChoice} {
		t.Run(name, func(t *testing.T) {
			_, err := p.Generate(context.Background(), req)
			if !errors.Is(err, fault.InvalidInput) {
				t.Fatalf("Generate() = %v, want fault.InvalidInput", err)
			}
		})
	}
	if captured.calls != 0 {
		t.Fatalf("made %d HTTP calls, want 0", captured.calls)
	}
}

func TestHTTPErrorsAreClassifiedWithoutLeakingDetails(t *testing.T) {
	vendorBody := func(msg string) string {
		return `{"error":{"message":"` + msg + ` (key sk-test-secret)","type":"invalid_request_error","code":"x"}}`
	}
	tests := []struct {
		name       string
		status     int
		body       string
		headers    map[string]string
		kind       fault.Kind
		retryAfter time.Duration
	}{
		{"bad request", 400, vendorBody("bad field"), nil, fault.InvalidInput, 0},
		{"context too large", 400, vendorBody("This model's maximum context length is 131072 tokens"), nil, fault.ContextTooLarge, 0},
		{"wrong key", 401, vendorBody("Authentication Fails"), nil, fault.Unauthenticated, 0},
		{"no balance", 402, vendorBody("Insufficient Balance"), nil, fault.QuotaExhausted, 0},
		{"forbidden", 403, vendorBody("forbidden"), nil, fault.Unauthorized, 0},
		{"invalid parameters", 422, vendorBody("bad params"), nil, fault.InvalidInput, 0},
		{"rate limited", 429, vendorBody("slow down"), map[string]string{"Retry-After": "7"}, fault.RateLimited, 7 * time.Second},
		{"server error", 500, vendorBody("oops"), nil, fault.TemporaryFailure, 0},
		{"overloaded", 503, "Service Unavailable", nil, fault.ProviderUnavailable, 0},
		{"gateway timeout", 504, "", nil, fault.Timeout, 0},
		{"unexpected status", 418, "teapot", nil, fault.Internal, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newServer(t, tt.status, tt.body, tt.headers)

			_, err := p.Generate(context.Background(), userRequest())
			if got := fault.KindOf(err); got != tt.kind {
				t.Fatalf("kind = %q, want %q (err %v)", got, tt.kind, err)
			}
			if strings.Contains(err.Error(), "sk-test-secret") {
				t.Fatalf("error leaks vendor details: %q", err.Error())
			}
			if d, _ := fault.RetryAfter(err); d != tt.retryAfter {
				t.Fatalf("retry after = %v, want %v", d, tt.retryAfter)
			}
		})
	}
}

func TestStreamFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
		kind fault.Kind
	}{
		{"error event mid stream", sse(
			`{"id":"r","choices":[{"index":0,"delta":{"content":"par"}}]}`,
			`{"error":{"message":"internal","type":"server_error"}}`,
		), fault.StreamFailed},
		{"connection closed before done", sse(
			`{"id":"r","choices":[{"index":0,"delta":{"content":"par"}}]}`,
		), fault.StreamFailed},
		{"server out of resources", sse(
			`{"id":"r","choices":[{"index":0,"delta":{},"finish_reason":"insufficient_system_resource"}]}`,
			`[DONE]`,
		), fault.ProviderUnavailable},
		{"malformed chunk", sse(`{not json`), fault.StreamFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newServer(t, http.StatusOK, tt.body, nil)
			_, err := p.Generate(context.Background(), userRequest())
			if got := fault.KindOf(err); got != tt.kind {
				t.Fatalf("kind = %q, want %q (err %v)", got, tt.kind, err)
			}
		})
	}
}

func TestFinishReasonMapping(t *testing.T) {
	tests := map[string]provider.FinishReason{
		"stop":           provider.FinishStop,
		"length":         provider.FinishLength,
		"content_filter": provider.FinishContentFilter,
		"something_new":  provider.FinishOther,
	}
	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			p, _ := newServer(t, http.StatusOK, sse(
				`{"id":"r","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":"`+raw+`"}]}`,
				`[DONE]`,
			), nil)
			resp, err := p.Generate(context.Background(), userRequest())
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if resp.FinishReason != want {
				t.Fatalf("finish = %q, want %q", resp.FinishReason, want)
			}
		})
	}
}

func TestStreamStopsWhenConsumerBreaksOrContextIsCanceled(t *testing.T) {
	released := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse(`{"id":"r","choices":[{"index":0,"delta":{"content":"first "}}]}`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		released <- struct{}{}
	}))
	t.Cleanup(server.Close)
	p, err := deepseek.New(deepseek.Config{APIKey: testKey, BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	for _, err := range p.Stream(context.Background(), userRequest()) {
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		break
	}
	waitReleased(t, released)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var streamErr error
	for _, err := range p.Stream(ctx, userRequest()) {
		if err != nil {
			streamErr = err
			break
		}
		cancel()
	}
	if !errors.Is(streamErr, context.Canceled) {
		t.Fatalf("stream error = %v, want context.Canceled", streamErr)
	}
	waitReleased(t, released)
}

func waitReleased(t *testing.T, released <-chan struct{}) {
	t.Helper()
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("the HTTP request was not released")
	}
}

func TestCapabilities(t *testing.T) {
	p, err := deepseek.New(deepseek.Config{APIKey: testKey})
	if err != nil {
		t.Fatal(err)
	}
	caps := p.Capabilities("deepseek-flash")
	if !caps.Tools || !caps.Reasoning || !caps.PromptCaching {
		t.Fatalf("capabilities = %+v", caps)
	}
	if caps.Vision || caps.Files || caps.StructuredOutput || caps.ForcedToolChoice {
		t.Fatalf("DeepSeek must not claim vision, files, schema-constrained output or forced tool choice: %+v", caps)
	}
}
