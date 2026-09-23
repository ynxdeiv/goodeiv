package provider_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/ynxdeiv/goodeiv/fault"
	"github.com/ynxdeiv/goodeiv/provider"
	"github.com/ynxdeiv/goodeiv/provider/providertest"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := provider.NewRegistry()
	fake := providertest.New()

	if err := registry.Register("fake", fake); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := registry.Get("fake")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != provider.Provider(fake) {
		t.Fatal("Get returned a different provider")
	}
}

func TestRegistryRejectsInvalidRegistrations(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("fake", providertest.New()); err != nil {
		t.Fatalf("Register: %v", err)
	}

	tests := []struct {
		name     string
		provName string
		prov     provider.Provider
	}{
		{"empty name", "", providertest.New()},
		{"nil provider", "other", nil},
		{"duplicate name", "fake", providertest.New()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(tt.provName, tt.prov)
			if !errors.Is(err, fault.InvalidInput) {
				t.Fatalf("Register() = %v, want fault.InvalidInput", err)
			}
		})
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	_, err := provider.NewRegistry().Get("missing")
	if !errors.Is(err, fault.InvalidInput) {
		t.Fatalf("Get() = %v, want fault.InvalidInput", err)
	}
}

func TestRegistryNamesAreSorted(t *testing.T) {
	registry := provider.NewRegistry()
	for _, name := range []string{"openai", "anthropic", "gemini"} {
		if err := registry.Register(name, providertest.New()); err != nil {
			t.Fatalf("Register(%q): %v", name, err)
		}
	}

	if got, want := registry.Names(), []string{"anthropic", "gemini", "openai"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}

func TestRegistryIsSafeForConcurrentUse(t *testing.T) {
	registry := provider.NewRegistry()
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = registry.Register(string(rune('a'+i%26))+"-p", providertest.New())
		}()
		go func() {
			defer wg.Done()
			_ = registry.Names()
			_, _ = registry.Get("a-p")
		}()
	}
	wg.Wait()
}
