package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/queue"
	"github.com/yourname/jobscheduler/internal/repository"
)

// RunService handles run business logic.
type RunService struct {
	jobRepo   repository.JobRepository
	runRepo   repository.RunRepository
	publisher *queue.Publisher
}

// NewRunService creates a RunService.
func NewRunService(
	jobRepo repository.JobRepository,
	runRepo repository.RunRepository,
	publisher *queue.Publisher,
) *RunService {
	return &RunService{jobRepo: jobRepo, runRepo: runRepo, publisher: publisher}
}

// TriggerJob creates a run for the given job and publishes it to NATS.
// Returns the created run. (FR-04)
func (s *RunService) TriggerJob(ctx context.Context, jobID, ownerID string) (*domain.Run, error) {
	job, err := s.jobRepo.GetByID(ctx, jobID, ownerID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job not found")
	}
	if job.Status == domain.JobStatusPaused {
		return nil, fmt.Errorf("job is paused")
	}

	now := time.Now().UTC()
	run := &domain.Run{
		ID:           ulid.Make().String(),
		JobID:        job.ID,
		OwnerID:      ownerID,
		ScheduledFor: now,
		State:        domain.RunStateQueued,
		Attempt:      1,
		ExpiresAt:    now.AddDate(0, 0, 30), // NFR-11: 30-day TTL
	}

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	// Publish to NATS. If this fails, the run stays queued.
	// The reaper will republish orphaned queued runs (ADR-11).
	msg := queue.RunMessage{
		RunID:   run.ID,
		JobID:   job.ID,
		Attempt: 1,
	}
	if err := s.publisher.PublishRun(ctx, msg, job.Policy.Priority); err != nil {
		// Log the error but don't fail the request — reaper will recover.
		return run, nil
	}
	return run, nil
}

// GetRun returns a run by ID for the given owner.
func (s *RunService) GetRun(ctx context.Context, runID, ownerID string) (*domain.Run, error) {
	run, err := s.runRepo.GetByID(ctx, runID, ownerID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, nil
	}
	return run, nil
}

// ListRuns returns paginated runs for a job.
func (s *RunService) ListRuns(ctx context.Context, f repository.ListRunsFilter) ([]*domain.Run, string, error) {
	return s.runRepo.List(ctx, f)
}

// RetryDeadRun creates a new run linked to the original dead run. (FR-07)
func (s *RunService) RetryDeadRun(ctx context.Context, runID, ownerID string) (*domain.Run, error) {
	original, err := s.runRepo.GetByID(ctx, runID, ownerID)
	if err != nil || original == nil {
		return nil, fmt.Errorf("run not found")
	}
	if original.State != domain.RunStateDead && original.State != domain.RunStateFailed {
		return nil, fmt.Errorf("only dead or failed runs can be retried")
	}

	job, err := s.jobRepo.GetByID(ctx, original.JobID, ownerID)
	if err != nil || job == nil {
		return nil, fmt.Errorf("job not found")
	}

	now := time.Now().UTC()
	newRun := &domain.Run{
		ID:           ulid.Make().String(),
		JobID:        original.JobID,
		OwnerID:      ownerID,
		ParentRunID:  original.ID,
		ScheduledFor: now,
		State:        domain.RunStateQueued,
		Attempt:      1,
		ExpiresAt:    now.AddDate(0, 0, 30),
	}

	if err := s.runRepo.Create(ctx, newRun); err != nil {
		return nil, fmt.Errorf("create retry run: %w", err)
	}

	msg := queue.RunMessage{RunID: newRun.ID, JobID: job.ID, Attempt: 1}
	_ = s.publisher.PublishRun(ctx, msg, job.Policy.Priority)

	return newRun, nil
}
