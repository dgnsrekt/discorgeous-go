package tts

import (
	"errors"
	"sync"
)

var (
	// ErrEngineNotFound is returned when an engine is not registered.
	ErrEngineNotFound = errors.New("TTS engine not found")
	// ErrEngineExists is returned when trying to register a duplicate engine.
	ErrEngineExists = errors.New("TTS engine already registered")
)

// Registry manages available TTS engines.
type Registry struct {
	mu      sync.RWMutex
	engines map[string]Engine
	def     string
}

// NewRegistry creates a new TTS engine registry.
func NewRegistry() *Registry {
	return &Registry{
		engines: make(map[string]Engine),
	}
}

// Register adds an engine to the registry.
func (r *Registry) Register(engine Engine) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := engine.Name()
	if _, exists := r.engines[name]; exists {
		return ErrEngineExists
	}

	r.engines[name] = engine

	// Set as default if first engine
	if r.def == "" {
		r.def = name
	}

	return nil
}

// Get retrieves an engine by name.
func (r *Registry) Get(name string) (Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engine, exists := r.engines[name]
	if !exists {
		return nil, ErrEngineNotFound
	}

	return engine, nil
}

// Default returns the default engine.
func (r *Registry) Default() (Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.def == "" {
		return nil, ErrEngineNotFound
	}

	return r.engines[r.def], nil
}

// SetDefault sets the default engine by name.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.engines[name]; !exists {
		return ErrEngineNotFound
	}

	r.def = name
	return nil
}

// List returns all registered engine names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.engines))
	for name := range r.engines {
		names = append(names, name)
	}
	return names
}
