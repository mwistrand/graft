package provider

import (
	"context"
	"testing"
)

// testProvider is a minimal provider implementation for testing.
type testProvider struct {
	name string
}

func (p *testProvider) Name() string { return p.name }
func (p *testProvider) SummarizeChanges(ctx context.Context, req *SummarizeRequest) (*SummarizeResponse, error) {
	return &SummarizeResponse{Overview: "test"}, nil
}
func (p *testProvider) OrderFiles(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
	return &OrderResponse{Reasoning: "test"}, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry("default")

	p1 := &testProvider{name: "provider1"}
	p2 := &testProvider{name: "provider2"}

	r.Register(p1)
	r.Register(p2)

	// Get by name
	got, err := r.Get("provider1")
	if err != nil {
		t.Fatalf("Get(provider1) failed: %v", err)
	}
	if got.Name() != "provider1" {
		t.Errorf("Name() = %q, want %q", got.Name(), "provider1")
	}

	got, err = r.Get("provider2")
	if err != nil {
		t.Fatalf("Get(provider2) failed: %v", err)
	}
	if got.Name() != "provider2" {
		t.Errorf("Name() = %q, want %q", got.Name(), "provider2")
	}
}

func TestRegistryDefault(t *testing.T) {
	r := NewRegistry("provider1")

	p1 := &testProvider{name: "provider1"}
	r.Register(p1)

	// Get with empty name returns default
	got, err := r.Get("")
	if err != nil {
		t.Fatalf("Get('') failed: %v", err)
	}
	if got.Name() != "provider1" {
		t.Errorf("Name() = %q, want %q", got.Name(), "provider1")
	}

	// Default() returns same
	got, err = r.Default()
	if err != nil {
		t.Fatalf("Default() failed: %v", err)
	}
	if got.Name() != "provider1" {
		t.Errorf("Name() = %q, want %q", got.Name(), "provider1")
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	r := NewRegistry("default")

	r.Register(&testProvider{name: "known"})

	_, err := r.Get("unknown")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestRegistrySetDefault(t *testing.T) {
	r := NewRegistry("p1")

	r.Register(&testProvider{name: "p1"})
	r.Register(&testProvider{name: "p2"})

	if r.DefaultName() != "p1" {
		t.Errorf("DefaultName() = %q, want %q", r.DefaultName(), "p1")
	}

	err := r.SetDefault("p2")
	if err != nil {
		t.Fatalf("SetDefault(p2) failed: %v", err)
	}

	if r.DefaultName() != "p2" {
		t.Errorf("DefaultName() = %q, want %q", r.DefaultName(), "p2")
	}
}

func TestRegistrySetDefaultUnregistered(t *testing.T) {
	r := NewRegistry("p1")

	err := r.SetDefault("unregistered")
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestRegistryHas(t *testing.T) {
	r := NewRegistry("default")

	r.Register(&testProvider{name: "exists"})

	if !r.Has("exists") {
		t.Error("Has(exists) = false, want true")
	}

	if r.Has("missing") {
		t.Error("Has(missing) = true, want false")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry("default")

	r.Register(&testProvider{name: "charlie"})
	r.Register(&testProvider{name: "alpha"})
	r.Register(&testProvider{name: "bravo"})

	list := r.List()

	// Should be sorted
	if len(list) != 3 {
		t.Fatalf("List() returned %d items, want 3", len(list))
	}

	expected := []string{"alpha", "bravo", "charlie"}
	for i, name := range expected {
		if list[i] != name {
			t.Errorf("List()[%d] = %q, want %q", i, list[i], name)
		}
	}
}

func TestRegistryEmpty(t *testing.T) {
	r := NewRegistry("default")

	_, err := r.Get("anything")
	if err == nil {
		t.Error("expected error for empty registry")
	}
}
