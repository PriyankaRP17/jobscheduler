package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/queue"
	"github.com/yourname/jobscheduler/internal/repository"
	"github.com/yourname/jobscheduler/pkg/backoff"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	fetchBatch        = 5               // messages per fetch
	fetchTimeout      = 2 * time.Second // how long to wait for messages
	shutdownDrainWait = 30 * time.Second
)

// Worker pulls runs from NATS and executes them.
type Worker struct {
	id        string
	consumer  *queue.Consumer
	publisher *queue.Publisher
	jobRepo   repository.JobRepository
	runRepo   repository.RunRepository
	executor  *Executor
	log       *slog.Logger
	wg        sync.WaitGroup
}

// New creates a Worker.
func New(
	id string,
	consumer *queue.Consumer,
	publisher *queue.Publisher,
	jobRepo repository.JobRepository,
	runRepo repository.RunRepository,
	executor *Executor,
	log *slog.Logger,
) *Worker {
	return &Worker{
		id:        id,
		consumer:  consumer,
		publisher: publisher,
		jobRepo:   jobRepo,
		runRepo:   runRepo,
		executor:  executor,
		log:       log,
	}
}

// Run starts the pull loop. It exits when ctx is cancelled.
// On shutdown it drains in-flight work up to shutdownDrainWait.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("worker started", "worker_id", w.id)
	for {
		// Stop fetching new work when shutting down.
		if ctx.Err() != nil {
			break
		}

		batch, err := w.consumer.FetchWithTimeout(fetchBatch, fetchTimeout)
		if err != nil {
			// Timeout is expected when the queue is empty.
			if ctx.Err() != nil {
				break
			}
			continue
		}

		for msg := range batch.Messages() {
			if ctx.Err() != nil {
				// Return the message so another worker can pick it up.
				_ = msg.Nak()
				continue
			}
			w.wg.Add(1)
			go func(m jetstream.Msg) {
				defer w.wg.Done()
				w.handle(context.Background(), m) // background: don't cancel mid-job
			}(msg)
		}
	}

	// Wait for in-flight jobs to finish.
	done := make(chan struct{})
	go func() { w.wg.Wait(); close(done) }()
	select {
	case <-done:
		w.log.Info("worker drained cleanly", "worker_id", w.id)
	case <-time.After(shutdownDrainWait):
		w.log.Warn("worker drain timeout, some jobs may be redelivered", "worker_id", w.id)
	}
}

// handle processes one NATS message end-to-end.
func (w *Worker) handle(ctx context.Context, msg jetstream.Msg) {
	var rm queue.RunMessage
	if err := json.Unmarshal(msg.Data(), &rm); err != nil {
		w.log.Error("bad message, acking to discard", "err", err)
		_ = msg.Ack()
		return
	}

	log := w.log.With("run_id", rm.RunID, "job_id", rm.JobID, "attempt", rm.Attempt)

	// Load run and job from MongoDB.
	run, err := w.runRepo.GetByIDOnly(ctx, rm.RunID)
	if err != nil || run == nil {
		log.Error("run not found, acking to discard", "err", err)
		_ = msg.Ack()
		return
	}

	// If already terminal (duplicate delivery), ack and skip.
	if domain.IsTerminal(run.State) {
		log.Info("run already terminal, skipping duplicate", "state", run.State)
		_ = msg.Ack()
		return
	}

	job, err := w.jobRepo.GetByID(ctx, run.JobID, run.OwnerID)
	if err != nil || job == nil {
		log.Error("job not found, acking to discard")
		_ = msg.Ack()
		return
	}

	// Transition to running (conditional, safe if another worker got here first).
	if err := w.runRepo.MarkRunning(ctx, run.ID, w.id); err != nil {
		log.Error("mark running failed", "err", err)
		_ = msg.Nak()
		return
	}

	// Publish run.started event for the SSE hub.
	_ = w.publisher.PublishEvent(ctx, queue.RunEventMessage{
		RunID: run.ID, JobID: run.JobID, OwnerID: run.OwnerID,
		State: string(domain.RunStateRunning), Attempt: run.Attempt,
	})

	log.Info("executing job")
	result := w.executor.Execute(ctx, run, job)

	attemptLog := domain.AttemptLog{
		N:          run.Attempt,
		At:         time.Now().UTC(),
		HTTPStatus: result.HTTPStatus,
		DurationMs: result.DurationMs,
	}
	if result.Error != nil {
		attemptLog.Error = result.Error.Error()
	}

	if result.Error == nil {
		// ── SUCCESS ──────────────────────────────────────────────────────────
		if err := w.runRepo.MarkFinished(ctx, run.ID, domain.RunStateSucceeded,
			result.Output, "", result.HTTPStatus, result.DurationMs, attemptLog); err != nil {
			log.Error("mark succeeded failed", "err", err)
			// Don't ack yet; redelivery will try again.
			_ = msg.Nak()
			return
		}
		_ = w.publisher.PublishEvent(ctx, queue.RunEventMessage{
			RunID: run.ID, JobID: run.JobID, OwnerID: run.OwnerID,
			State: string(domain.RunStateSucceeded), Attempt: run.Attempt,
		})
		log.Info("job succeeded", "duration_ms", result.DurationMs)
		// Ack ONLY after result is persisted (ADR-04 / NFR-01).
		_ = msg.Ack()

	} else {
		// ── FAILURE ──────────────────────────────────────────────────────────
		nextAttempt := run.Attempt + 1
		if run.Attempt > job.Policy.MaxRetries {
			// Retries exhausted → dead.
			if err := w.runRepo.MarkFinished(ctx, run.ID, domain.RunStateDead,
				result.Output, result.Error.Error(), result.HTTPStatus, result.DurationMs, attemptLog); err != nil {
				log.Error("mark dead failed", "err", err)
				_ = msg.Nak()
				return
			}
			_ = w.publisher.PublishDead(ctx, rm)
			_ = w.publisher.PublishEvent(ctx, queue.RunEventMessage{
				RunID: run.ID, JobID: run.JobID, OwnerID: run.OwnerID,
				State: string(domain.RunStateDead), Attempt: run.Attempt,
				Error: result.Error.Error(),
			})
			log.Warn("job dead after max retries", "max_retries", job.Policy.MaxRetries)
			_ = msg.Ack() // ack so it leaves the JOBS stream
		} else {
			// Retry with backoff.
			delay := backoff.Delay(run.Attempt)
			if err := w.runRepo.MarkRetrying(ctx, run.ID, nextAttempt, attemptLog); err != nil {
				log.Error("mark retrying failed", "err", err)
				_ = msg.Nak()
				return
			}
			_ = w.publisher.PublishEvent(ctx, queue.RunEventMessage{
				RunID: run.ID, JobID: run.JobID, OwnerID: run.OwnerID,
				State: string(domain.RunStateRetrying), Attempt: run.Attempt,
				Error: result.Error.Error(),
			})
			log.Warn("job failed, retrying", "next_attempt", nextAttempt, "delay", delay)
			_ = msg.NakWithDelay(delay)
		}
	}
}
