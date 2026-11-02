// Package tools defines the Tool interface and a Registry that maps tool
// names to implementations for the agent loop.
package tools

import (
	"context"
	"fmt"
)

// Tool is a function the model may call. Exec receives the raw JSON-encoded
// argument string from the model and returns a text result.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Exec(ctx context.Context, argsJSON string) (string, error)
}

// Registry maps tool names to Tool implementations.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns the tool registered under name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names returns the names of all registered tools.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Exec looks up name and invokes it with argsJSON. Unknown tools return an
// error rather than panicking.
func (r *Registry) Exec(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %s", name)
	}
	return t.Exec(ctx, argsJSON)
}
