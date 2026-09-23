package providertest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/message"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/provider/providertest"
	"github.com/ynxdeiv/goodeiv/usage"
)

func request(text string) provider.Request {
	return provider.Request{Model: "fake-model", Messages: []message.Message{message.User(text)}}
}

func TestScriptedTurnsAreConsumedInOrder(t *testing.T) {
	call := message.ToolCall{ID: "c1", Name: "weather.current", Arguments: json.RawMessage(`{"city":"Lisbon"}`)}
	fake := providertest.New(
		providertest.CallTools(call),
		providertest.Reply("It is sunny."),
	)
	ctx := context.Background()

	first, err := fake.Generate(ctx, request("weather?"))
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	if calls := first.Message.ToolCalls(); len(calls) != 1 || calls[0].ID != "c1" {
		t.Fatalf("first turn tool calls = %+v", calls)
	}
	if first.FinishReason != provider.FinishToolCalls {
		t.Fatalf("first finish = %q", first.FinishReason)
	}

	second, err := fake.Generate(ctx, request("and now?"))
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if second.Message.Text() != "It is sunny." || second.FinishReason != provider.FinishStop {
		t.Fatalf("second turn = %+v", second)
	}
	if second.Model != "fake-model" {
		t.Fatalf("model = %q", second.Model)
	}

	if fake.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want 0", fake.Remaining())
	}
	if got := fake.Requests(); len(got) != 2 || got[1].Messages[0].Text() != "and now?" {
		t.Fatalf("Requests() = %+v", got)
	}
}

func TestExhaustedScriptFails(t *testing.T) {
	fake := providertest.New()
	_, err := fake.Generate(context.Background(), request("hi"))
	if !errors.Is(err, fault.Internal) {
		t.Fatalf("Generate() = %v, want fault.Internal", err)
	}
}

func TestFailTurnReturnsTheScriptedError(t *testing.T) {
	limited := fault.New(fault.RateLimited, "slow down")
	fake := providertest.New(providertest.Fail(limited))

	_, err := fake.Generate(context.Background(), request("hi"))
	if !errors.Is(err, limited) {
		t.Fatalf("Generate() = %v, want %v", err, limited)
	}
}

func TestInterruptFailsAfterPartialOutput(t *testing.T) {
	broken := fault.New(fault.StreamFailed, "connection reset")
	fake := providertest.New(providertest.Interrupt("partial answer", broken))

	var text string
	var streamErr error
	for ev, err := range fake.Stream(context.Background(), request("hi")) {
		if err != nil {
			streamErr = err
			break
		}
		text += ev.Delta
	}

	if text != "partial answer" {
		t.Fatalf("text before failure = %q", text)
	}
	if !errors.Is(streamErr, broken) {
		t.Fatalf("stream error = %v, want %v", streamErr, broken)
	}
}

func TestInvalidRequestIsRejectedWithoutConsumingATurn(t *testing.T) {
	fake := providertest.New(providertest.Reply("hi"))

	_, err := fake.Generate(context.Background(), provider.Request{})
	if !errors.Is(err, fault.InvalidInput) {
		t.Fatalf("Generate() = %v, want fault.InvalidInput", err)
	}
	if fake.Remaining() != 1 {
		t.Fatal("an invalid request must not consume a scripted turn")
	}
}

func TestStreamHonorsCanceledContext(t *testing.T) {
	fake := providertest.New(providertest.Reply("never sent"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fake.Generate(ctx, request("hi"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() = %v, want context.Canceled", err)
	}
}

func TestStreamStopsWhenConsumerBreaks(t *testing.T) {
	fake := providertest.New(providertest.Reply("one two three four"))

	received := 0
	for _, err := range fake.Stream(context.Background(), request("hi")) {
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		received++
		break
	}
	if received != 1 {
		t.Fatalf("received %d events after break, want 1", received)
	}
}

func TestUsageIsReported(t *testing.T) {
	u := usage.Usage{InputTokens: usage.Known(10), OutputTokens: usage.Known(3)}
	fake := providertest.New(providertest.Reply("ok").WithUsage(u))

	resp, err := fake.Generate(context.Background(), request("hi"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Usage != u {
		t.Fatalf("usage = %+v, want %+v", resp.Usage, u)
	}
}

func TestCapabilitiesDefaultToEverything(t *testing.T) {
	caps := providertest.New().Capabilities("any")
	if !caps.Tools || !caps.Vision || !caps.Files {
		t.Fatalf("default capabilities = %+v", caps)
	}

	limited := providertest.New().WithCapabilities(provider.Capabilities{})
	if limited.Capabilities("any").Tools {
		t.Fatal("WithCapabilities was ignored")
	}
}

func Example() {
	fake := providertest.New(providertest.Reply("Hello from the fake provider."))

	resp, err := fake.Generate(context.Background(), provider.Request{
		Model:    "any-model",
		Messages: []message.Message{message.User("Hi!")},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.Message.Text())
	fmt.Println(resp.FinishReason)
	// Output:
	// Hello from the fake provider.
	// stop
}
