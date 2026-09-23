package provider

import (
	"slices"
	"sync"
)

// Registry maps provider names to providers. It is safe for concurrent use and has no global
// instance: create one with NewRegistry and pass it where it is needed.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}}
}

// Register adds p under name. It fails with fault.InvalidInput if name is empty, p is nil or
// name is already registered.
func (r *Registry) Register(name string, p Provider) error {
	if name == "" {
		return invalid("provider name is empty")
	}
	if p == nil {
		return invalid("provider %q is nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[name]; exists {
		return invalid("provider %q is already registered", name)
	}
	r.providers[name] = p
	return nil
}

// Get returns the provider registered under name, or a fault.InvalidInput error.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, invalid("provider %q is not registered", name)
	}
	return p, nil
}

// Names returns the registered provider names in ascending order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
