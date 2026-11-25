package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/repository"
)

// JobService handles job business logic.
type JobService struct {
	jobRepo repository.JobRepository
}

// NewJobService creates a JobService.
func NewJobService(jobRepo repository.JobRepository) *JobService {
	return &JobService{jobRepo: jobRepo}
}

// CreateJobInput is the input for creating a job.
type CreateJobInput struct {
	OwnerID  string
	Name     string
	Type     string
	Payload  map[string]any
	Schedule domain.Schedule
	Policy   domain.RetryPolicy
}

// CreateJob validates and persists a new job.
func (s *JobService) CreateJob(ctx context.Context, input CreateJobInput) (*domain.Job, error) {
	now := time.Now().UTC()
	job := &domain.Job{
		ID:        ulid.Make().String(),
		OwnerID:   input.OwnerID,
		Name:      input.Name,
		Type:      input.Type,
		Payload:   input.Payload,
		Schedule:  input.Schedule,
		Policy:    input.Policy,
		Status:    domain.JobStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Set default timeout if not provided.
	if job.Policy.TimeoutSec == 0 {
		job.Policy.TimeoutSec = 30
	}
	if job.Policy.Priority == "" {
		job.Policy.Priority = "default"
	}

	if err := job.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	// Compute next_run_at for cron/delay/once schedules.
	nextRunAt, err := computeNextRun(job.Schedule, now)
	if err != nil {
		return nil, fmt.Errorf("schedule: %w", err)
	}
	job.NextRunAt = nextRunAt

	if err := s.jobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	return job, nil
}

// GetJob returns a job by ID for a given owner.
func (s *JobService) GetJob(ctx context.Context, id, ownerID string) (*domain.Job, error) {
	job, err := s.jobRepo.GetByID(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil // handler returns 404
	}
	return job, nil
}

// ListJobs returns paginated jobs for an owner.
func (s *JobService) ListJobs(ctx context.Context, f repository.ListJobsFilter) ([]*domain.Job, string, error) {
	return s.jobRepo.List(ctx, f)
}

// PauseJob sets a job's status to paused.
func (s *JobService) PauseJob(ctx context.Context, id, ownerID string) error {
	job, err := s.jobRepo.GetByID(ctx, id, ownerID)
	if err != nil || job == nil {
		return fmt.Errorf("job not found")
	}
	job.Status = domain.JobStatusPaused
	job.UpdatedAt = time.Now().UTC()
	return s.jobRepo.Update(ctx, job)
}

// ResumeJob sets a job's status back to active and recomputes next_run_at.
func (s *JobService) ResumeJob(ctx context.Context, id, ownerID string) error {
	job, err := s.jobRepo.GetByID(ctx, id, ownerID)
	if err != nil || job == nil {
		return fmt.Errorf("job not found")
	}
	now := time.Now().UTC()
	nextRunAt, err := computeNextRun(job.Schedule, now)
	if err != nil {
		return err
	}
	job.Status = domain.JobStatusActive
	job.NextRunAt = nextRunAt
	job.UpdatedAt = now
	return s.jobRepo.Update(ctx, job)
}

// DeleteJob removes a job.
func (s *JobService) DeleteJob(ctx context.Context, id, ownerID string) error {
	return s.jobRepo.Delete(ctx, id, ownerID)
}

// computeNextRun calculates the first run time for any schedule kind.
// Week 2 will replace the cron stub with a real cron parser.
func computeNextRun(s domain.Schedule, now time.Time) (*time.Time, error) {
	switch s.Kind {
	case domain.ScheduleOnce, domain.ScheduleDelay:
		if s.RunAt == nil {
			return nil, fmt.Errorf("run_at is required")
		}
		t := s.RunAt.UTC()
		return &t, nil
	case domain.ScheduleCron:
		// Stub for Week 1 — cron parsing added in Week 2.
		// Returns now+1 min so the scheduler can fire it in tests.
		t := now.Add(1 * time.Minute)
		return &t, nil
	default:
		return nil, fmt.Errorf("unknown schedule kind: %s", s.Kind)
	}
}
