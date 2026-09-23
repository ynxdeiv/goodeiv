package chatcompletions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/provider"
)

type Config struct {
	Name          string
	Endpoint      string
	APIKey        string
	HTTPClient    *http.Client
	StreamUsage   bool
	EchoReasoning bool
}

type Client struct {
	cfg Config
}

func New(cfg Config) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Stream(ctx context.Context, req provider.Request) iter.Seq2[provider.Event, error] {
	return func(yield func(provider.Event, error) bool) {
		names, err := newToolNames(req)
		if err != nil {
			yield(provider.Event{}, err)
			return
		}
		resp, err := c.send(ctx, req, names)
		if err != nil {
			yield(provider.Event{}, err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		c.readStream(ctx, resp.Body, newStreamParser(c.cfg.Name, names), yield)
	}
}

func (c *Client) send(ctx context.Context, req provider.Request, names toolNames) (*http.Response, error) {
	body, err := c.buildRequest(req, names)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fault.Wrap(fault.Internal, c.cfg.Name+": encoding request", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fault.Wrap(fault.InvalidInput, c.cfg.Name+": building HTTP request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, transportError(ctx, c.cfg.Name, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		return nil, statusError(c.cfg.Name, resp)
	}
	return resp, nil
}

func (c *Client) readStream(ctx context.Context, body io.Reader, parser *streamParser, yield func(provider.Event, error) bool) {
	reader := bufio.NewReader(body)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			events, done, err := parser.line(strings.TrimRight(line, "\r\n"))
			for _, ev := range events {
				if !yield(ev, nil) {
					return
				}
			}
			if err != nil {
				yield(provider.Event{}, err)
				return
			}
			if done {
				return
			}
		}
		if readErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				yield(provider.Event{}, ctxErr)
				return
			}
			if errors.Is(readErr, io.EOF) {
				yield(provider.Event{}, fault.New(fault.StreamFailed, c.cfg.Name+": stream closed before completion"))
				return
			}
			yield(provider.Event{}, fault.Wrap(fault.StreamFailed, c.cfg.Name+": reading stream", readErr))
			return
		}
	}
}
