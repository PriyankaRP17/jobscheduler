package repository

import (
	"context"
	"time"

	"github.com/yourname/jobscheduler/internal/domain"
)

// ListJobsFilter is used when listing jobs.
type ListJobsFilter struct {
	OwnerID string
	Status  domain.JobStatus
	Limit   int
	Cursor  string // last seen ID for cursor pagination
}

// JobRepository defines all data access for jobs.
type JobRepository interface {
	Create(ctx context.Context, job *domain.Job) error
	GetByID(ctx context.Context, id, ownerID string) (*domain.Job, error)
	List(ctx context.Context, f ListJobsFilter) ([]*domain.Job, string, error)
	Update(ctx context.Context, job *domain.Job) error
	Delete(ctx context.Context, id, ownerID string) error

	// ClaimDue atomically claims a due job and updates next_run_at.
	// Returns nil if another instance claimed it first (NFR-02).
	ClaimDue(ctx context.Context, jobID string, oldNextRunAt time.Time, newNextRunAt *time.Time) (*domain.Job, error)

	// FindDue returns active jobs with next_run_at <= now (up to limit).
	FindDue(ctx context.Context, now time.Time, limit int) ([]*domain.Job, error)
}

// ListRunsFilter is used when listing runs.
type ListRunsFilter struct {
	JobID   string
	OwnerID string
	State   domain.RunState
	Limit   int
	Cursor  string
}

// RunRepository defines all data access for runs.
type RunRepository interface {
	Create(ctx context.Context, run *domain.Run) error
	GetByID(ctx context.Context, id, ownerID string) (*domain.Run, error)
	List(ctx context.Context, f ListRunsFilter) ([]*domain.Run, string, error)

	// MarkRunning transitions queued/retrying -> running (conditional update).
	MarkRunning(ctx context.Context, runID, workerID string) error

	// MarkFinished saves the final state and output.
	MarkFinished(ctx context.Context, runID string, state domain.RunState, output, errMsg string, httpStatus int, durationMs int64, log domain.AttemptLog) error

	// MarkRetrying transitions running -> retrying.
	MarkRetrying(ctx context.Context, runID string, attempt int, log domain.AttemptLog) error

	// FindStuckQueued returns queued runs older than threshold with no worker.
	FindStuckQueued(ctx context.Context, olderThan time.Duration) ([]*domain.Run, error)

	// FindStuckRunning returns running runs whose worker is offline.
	FindStuckRunning(ctx context.Context, workerIDs []string) ([]*domain.Run, error)
	GetByIDOnly(ctx context.Context, id string) (*domain.Run, error)
}

// WorkerRepository tracks live worker instances.
type WorkerRepository interface {
	Upsert(ctx context.Context, w *domain.Worker) error
	GetByID(ctx context.Context, id string) (*domain.Worker, error)
	ListAll(ctx context.Context) ([]*domain.Worker, error)
	MarkOffline(ctx context.Context, id string) error
	FindDeadWorkers(ctx context.Context, heartbeatOlderThan time.Duration) ([]*domain.Worker, error)
}
