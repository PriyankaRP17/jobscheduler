package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Consumer wraps a durable NATS pull consumer.
type Consumer struct {
	c jetstream.Consumer
}

// NewConsumer creates or updates the durable pull consumer for workers.
// AckWait must be > max job timeout to avoid false redelivery (ADR-04).
func NewConsumer(js jetstream.JetStream, ackWait time.Duration) (*Consumer, error) {
	cfg := jetstream.ConsumerConfig{
		Name:          ConsumerWorkers,
		Durable:       ConsumerWorkers,
		FilterSubject: SubjectRunAll,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       ackWait,
		MaxDeliver:    15,  // safety cap; per-job limit enforced by worker
		BackOff:       nil, // worker controls backoff via NakWithDelay
		MaxAckPending: 50,  // backpressure: max in-flight across all workers
	}
	c, err := js.CreateOrUpdateConsumer(context.Background(), StreamJobs, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}
	return &Consumer{c: c}, nil
}

// Fetch pulls up to maxMsgs messages, waiting up to fetchTimeout.
// The short timeout lets the caller check context cancellation frequently.
func (c *Consumer) Fetch(maxMsgs int, fetchTimeout time.Duration) (jetstream.MessageBatch, error) {
	return c.c.FetchNoWait(maxMsgs)
}

// FetchWithTimeout fetches with a deadline, useful for blocking pulls.
func (c *Consumer) FetchWithTimeout(maxMsgs int, timeout time.Duration) (jetstream.MessageBatch, error) {
	return c.c.Fetch(maxMsgs, jetstream.FetchMaxWait(timeout))
}
