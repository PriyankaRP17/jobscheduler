package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/worker/handlers"
)

// ExecuteResult is what the executor returns after one attempt.
type ExecuteResult struct {
	Output     string
	Error      error
	HTTPStatus int
	DurationMs int64
}

// Executor runs a job using the handler registry.
type Executor struct {
	registry *handlers.Registry
}

// NewExecutor creates an Executor.
func NewExecutor(registry *handlers.Registry) *Executor {
	return &Executor{registry: registry}
}

// Execute runs the job with a context deadline and returns the result.
func (e *Executor) Execute(parentCtx context.Context, run *domain.Run, job *domain.Job) ExecuteResult {
	timeout := time.Duration(job.Policy.TimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	h, err := e.registry.Get(job.Type)
	if err != nil {
		return ExecuteResult{Error: fmt.Errorf("handler: %w", err)}
	}

	start := time.Now()
	result, execErr := h.Execute(ctx, run.ID, job.Payload)
	durationMs := time.Since(start).Milliseconds()

	return ExecuteResult{
		Output:     result.Output,
		Error:      execErr,
		HTTPStatus: result.HTTPStatus,
		DurationMs: durationMs,
	}
}
