package handlers

import (
	"context"
	"fmt"
)

// Result holds the output of a handler execution.
type Result struct {
	Output     string
	HTTPStatus int // 0 for non-HTTP handlers
}

// Handler is the interface every job type must implement.
type Handler interface {
	Execute(ctx context.Context, runID string, payload map[string]any) (Result, error)
}

// Registry maps job types to handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds a handler for a job type.
func (r *Registry) Register(jobType string, h Handler) {
	r.handlers[jobType] = h
}

// Get returns the handler for the given type.
func (r *Registry) Get(jobType string) (Handler, error) {
	h, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for job type %q", jobType)
	}
	return h, nil
}
