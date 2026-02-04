package tts

import (
	"context"
	"errors"
	"testing"
)

// mockEngine is a test implementation of Engine.
type mockEngine struct {
	name string
}

func (m *mockEngine) Name() string {
	return m.name
}

func (m *mockEngine) Synthesize(ctx context.Context, req SynthesizeRequest) (*AudioResult, error) {
	return &AudioResult{
		Data:       []byte("mock audio"),
		Format:     "wav",
		SampleRate: 22050,
		Channels:   1,
	}, nil
}

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()
	engine := &mockEngine{name: "test"}

	err := reg.Register(engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify engine is registered
	got, err := reg.Get("test")
	if err != nil {
		t.Fatalf("failed to get engine: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", got.Name())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	reg := NewRegistry()
	engine := &mockEngine{name: "test"}

	if err := reg.Register(engine); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := reg.Register(engine)
	if !errors.Is(err, ErrEngineExists) {
		t.Errorf("expected ErrEngineExists, got %v", err)
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	if !errors.Is(err, ErrEngineNotFound) {
		t.Errorf("expected ErrEngineNotFound, got %v", err)
	}
}

func TestRegistry_Default(t *testing.T) {
	reg := NewRegistry()

	// No default initially
	_, err := reg.Default()
	if !errors.Is(err, ErrEngineNotFound) {
		t.Errorf("expected ErrEngineNotFound for empty registry, got %v", err)
	}

	// First engine becomes default
	engine1 := &mockEngine{name: "first"}
	if err := reg.Register(engine1); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	def, err := reg.Default()
	if err != nil {
		t.Fatalf("failed to get default: %v", err)
	}
	if def.Name() != "first" {
		t.Errorf("expected default 'first', got '%s'", def.Name())
	}

	// Second engine doesn't change default
	engine2 := &mockEngine{name: "second"}
	if err := reg.Register(engine2); err != nil {
		t.Fatalf("failed to register second: %v", err)
	}

	def, err = reg.Default()
	if err != nil {
		t.Fatalf("failed to get default after second register: %v", err)
	}
	if def.Name() != "first" {
		t.Errorf("expected default still 'first', got '%s'", def.Name())
	}
}

func TestRegistry_SetDefault(t *testing.T) {
	reg := NewRegistry()

	engine1 := &mockEngine{name: "first"}
	engine2 := &mockEngine{name: "second"}

	reg.Register(engine1)
	reg.Register(engine2)

	// Change default
	err := reg.SetDefault("second")
	if err != nil {
		t.Fatalf("failed to set default: %v", err)
	}

	def, err := reg.Default()
	if err != nil {
		t.Fatalf("failed to get default: %v", err)
	}
	if def.Name() != "second" {
		t.Errorf("expected default 'second', got '%s'", def.Name())
	}
}

func TestRegistry_SetDefaultNotFound(t *testing.T) {
	reg := NewRegistry()

	err := reg.SetDefault("nonexistent")
	if !errors.Is(err, ErrEngineNotFound) {
		t.Errorf("expected ErrEngineNotFound, got %v", err)
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()

	// Empty list
	names := reg.List()
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}

	// Add engines
	reg.Register(&mockEngine{name: "alpha"})
	reg.Register(&mockEngine{name: "beta"})
	reg.Register(&mockEngine{name: "gamma"})

	names = reg.List()
	if len(names) != 3 {
		t.Errorf("expected 3 engines, got %d", len(names))
	}

	// Check all names are present (order not guaranteed)
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	for _, expected := range []string{"alpha", "beta", "gamma"} {
		if !nameSet[expected] {
			t.Errorf("missing engine '%s' in list", expected)
		}
	}
}
