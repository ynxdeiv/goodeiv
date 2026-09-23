// Package deepseek is the goodeiv adapter for the DeepSeek API.
//
// DeepSeek thinking models return their reasoning as message.Reasoning parts. When a request
// carries tools, that reasoning must be sent back on later turns; the adapter does this
// automatically as long as the conversation keeps the assistant messages it received.
package deepseek

import (
	"context"
	"iter"
	"net/http"
	"strings"
	"time"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/internal/chatcompletions"
	"github.com/ynxdeiv/goodeiv/provider"
)

// DefaultBaseURL is the DeepSeek API root used when Config.BaseURL is empty.
const DefaultBaseURL = "https://api.deepseek.com"

// DefaultResponseHeaderTimeout bounds the wait for response headers when Config.HTTPClient is
// nil. Streams themselves are not cut by it; cancel the context to stop them.
const DefaultResponseHeaderTimeout = time.Minute

// Config configures the adapter.
type Config struct {
	// APIKey is the DeepSeek API key. Required.
	APIKey string
	// BaseURL overrides DefaultBaseURL, for proxies or tests.
	BaseURL string
	// HTTPClient overrides the default client. It must not set Client.Timeout, which would cut
	// long streams.
	HTTPClient *http.Client
}

// Provider implements provider.Provider for DeepSeek. It is safe for concurrent use.
type Provider struct {
	client *chatcompletions.Client
}

// New returns a DeepSeek provider. It fails with fault.InvalidInput if the API key is missing.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fault.New(fault.InvalidInput, "deepseek: API key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &Provider{client: chatcompletions.New(chatcompletions.Config{
		Name:          "deepseek",
		Endpoint:      strings.TrimRight(baseURL, "/") + "/chat/completions",
		APIKey:        cfg.APIKey,
		HTTPClient:    httpClient,
		StreamUsage:   true,
		EchoReasoning: true,
	})}, nil
}

// Generate returns the complete response for req.
func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	return provider.Collect(p.Stream(ctx, req))
}

// Stream validates req and streams normalized events. Invalid or unsupported requests fail
// before any network call.
func (p *Provider) Stream(ctx context.Context, req provider.Request) iter.Seq2[provider.Event, error] {
	if err := p.check(req); err != nil {
		return func(yield func(provider.Event, error) bool) {
			yield(provider.Event{}, err)
		}
	}
	return p.client.Stream(ctx, req)
}

// Capabilities reports DeepSeek features. Models are text-only thinking models with tool calling
// and automatic prompt caching. JSON mode exists but schema-constrained output does not, and
// thinking mode rejects forced tool choices (required or a named tool).
func (p *Provider) Capabilities(string) provider.Capabilities {
	return provider.Capabilities{
		Tools:             true,
		ParallelToolCalls: true,
		Reasoning:         true,
		PromptCaching:     true,
	}
}

func (p *Provider) check(req provider.Request) error {
	if err := req.Validate(); err != nil {
		return err
	}
	return p.Capabilities(req.Model).Supports(req)
}

func defaultHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = DefaultResponseHeaderTimeout
	return &http.Client{Transport: transport}
}
