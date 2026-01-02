package provider

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages available AI providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	defaultID string
}

// NewRegistry creates a new provider registry with the specified default provider ID.
func NewRegistry(defaultID string) *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		defaultID: defaultID,
	}
}

// Register adds a provider to the registry.
// If a provider with the same name already exists, it will be replaced.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
// If name is empty, returns the default provider.
// Returns an error if the provider is not found.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if name == "" {
		name = r.defaultID
	}

	p, ok := r.providers[name]
	if !ok {
		available := r.availableNames()
		if len(available) == 0 {
			return nil, fmt.Errorf("no providers registered")
		}
		return nil, fmt.Errorf("unknown provider %q; available: %v", name, available)
	}

	return p, nil
}

// Default returns the default provider.
// Returns an error if the default provider is not registered.
func (r *Registry) Default() (Provider, error) {
	return r.Get(r.defaultID)
}

// SetDefault changes the default provider ID.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider %q not registered", name)
	}

	r.defaultID = name
	return nil
}

// Has returns true if a provider with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[name]
	return ok
}

// List returns the names of all registered providers.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.availableNames()
}

// availableNames returns sorted provider names (must hold read lock).
func (r *Registry) availableNames() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultName returns the name of the default provider.
func (r *Registry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultID
}
