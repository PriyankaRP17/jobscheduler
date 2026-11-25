package domain

import (
	"errors"
	"time"
)

// RunState is the lifecycle state of one run.
type RunState string

const (
	RunStateQueued    RunState = "queued"
	RunStateRunning   RunState = "running"
	RunStateSucceeded RunState = "succeeded"
	RunStateRetrying  RunState = "retrying"
	RunStateFailed    RunState = "failed" // worker lost / reaper
	RunStateDead      RunState = "dead"   // retries exhausted
)

// AttemptLog records one attempt within a run.
type AttemptLog struct {
	N          int       `bson:"n" json:"n"`
	At         time.Time `bson:"at" json:"at"`
	HTTPStatus int       `bson:"http_status,omitempty" json:"http_status,omitempty"`
	Error      string    `bson:"error,omitempty" json:"error,omitempty"`
	DurationMs int64     `bson:"duration_ms" json:"duration_ms"`
}

// Run is one execution instance of a Job.
type Run struct {
	ID           string       `bson:"_id" json:"id"`
	JobID        string       `bson:"job_id" json:"job_id"`
	OwnerID      string       `bson:"owner_id" json:"owner_id"`
	ParentRunID  string       `bson:"parent_run_id,omitempty" json:"parent_run_id,omitempty"`
	ScheduledFor time.Time    `bson:"scheduled_for" json:"scheduled_for"`
	State        RunState     `bson:"state" json:"state"`
	Attempt      int          `bson:"attempt" json:"attempt"`
	WorkerID     string       `bson:"worker_id,omitempty" json:"worker_id,omitempty"`
	StartedAt    *time.Time   `bson:"started_at,omitempty" json:"started_at,omitempty"`
	FinishedAt   *time.Time   `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
	DurationMs   int64        `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"`
	HTTPStatus   int          `bson:"http_status,omitempty" json:"http_status,omitempty"`
	Output       string       `bson:"output,omitempty" json:"output,omitempty"`
	Error        string       `bson:"error,omitempty" json:"error,omitempty"`
	AttemptsLog  []AttemptLog `bson:"attempts_log,omitempty" json:"attempts_log,omitempty"`
	ExpiresAt    time.Time    `bson:"expires_at" json:"expires_at"`
}

// ValidTransition returns an error if moving from -> to is not allowed.
func ValidTransition(from, to RunState) error {
	allowed := map[RunState][]RunState{
		RunStateQueued:   {RunStateRunning},
		RunStateRunning:  {RunStateSucceeded, RunStateRetrying, RunStateDead, RunStateFailed},
		RunStateRetrying: {RunStateRunning},
		// Terminal states - no transitions out
		RunStateSucceeded: {},
		RunStateDead:      {},
		RunStateFailed:    {},
	}
	targets, ok := allowed[from]
	if !ok {
		return errors.New("unknown source state")
	}
	for _, t := range targets {
		if t == to {
			return nil
		}
	}
	return errors.New("invalid state transition: " + string(from) + " -> " + string(to))
}

// IsTerminal returns true if the state is a final state.
func IsTerminal(s RunState) bool {
	return s == RunStateSucceeded || s == RunStateDead || s == RunStateFailed
}
