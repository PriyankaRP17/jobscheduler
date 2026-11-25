package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// RunMessage is the small payload published to the JOBS stream.
// Workers load the full job from MongoDB to avoid large queue messages (ADR-03).
type RunMessage struct {
	RunID   string `json:"run_id"`
	JobID   string `json:"job_id"`
	Attempt int    `json:"attempt"`
}

// RunEventMessage is published to JOB_EVENTS for live dashboard updates.
type RunEventMessage struct {
	RunID   string `json:"run_id"`
	JobID   string `json:"job_id"`
	OwnerID string `json:"owner_id"`
	State   string `json:"state"`
	Attempt int    `json:"attempt"`
	Error   string `json:"error,omitempty"`
}

// Publisher publishes messages to NATS JetStream.
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher creates a Publisher.
func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{js: js}
}

// PublishRun enqueues a run for execution.
// Nats-Msg-Id is set to run_id for deduplication (ADR-04).
func (p *Publisher) PublishRun(ctx context.Context, msg RunMessage, priority string) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	subject := SubjectForPriority(priority)
	opts := []jetstream.PublishOpt{
		jetstream.WithMsgID(msg.RunID), // dedup key
	}
	ack, err := p.js.Publish(ctx, subject, data, opts...)
	if err != nil {
		return fmt.Errorf("publish run %s: %w", msg.RunID, err)
	}
	if ack.Duplicate {
		// Already published (scheduler retry) - safe to ignore.
		return nil
	}
	return nil
}

// PublishDead moves a run to the dead-letter stream.
func (p *Publisher) PublishDead(ctx context.Context, msg RunMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = p.js.Publish(ctx, SubjectDead, data,
		jetstream.WithMsgID("dead-"+msg.RunID))
	return err
}

// PublishEvent sends a run lifecycle event for the SSE hub.
func (p *Publisher) PublishEvent(ctx context.Context, ev RunEventMessage) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	subject := subjectForState(ev.State)
	_, err = p.js.Publish(ctx, subject, data)
	return err
}

func subjectForState(state string) string {
	m := map[string]string{
		"running":   SubjectEventStarted,
		"succeeded": SubjectEventSucceeded,
		"retrying":  SubjectEventRetrying,
		"failed":    SubjectEventFailed,
		"dead":      SubjectEventDead,
	}
	if s, ok := m[state]; ok {
		return s
	}
	return SubjectEventAll
}
