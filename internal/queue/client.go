package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client wraps a NATS connection and JetStream context.
type Client struct {
	Conn *nats.Conn
	JS   jetstream.JetStream
}

// Connect establishes a NATS connection and ensures all streams exist.
func Connect(url string) (*Client, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
		nats.Name("jobscheduler"),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	c := &Client{Conn: nc, JS: js}
	if err := c.ensureStreams(); err != nil {
		nc.Close()
		return nil, err
	}
	return c, nil
}

// Close drains and closes the NATS connection.
func (c *Client) Close() {
	_ = c.Conn.Drain()
}

func (c *Client) ensureStreams() error {
	streams := []jetstream.StreamConfig{
		{
			// Work queue: each message delivered to exactly one worker.
			Name:      StreamJobs,
			Subjects:  []string{SubjectRunAll},
			Retention: jetstream.WorkQueuePolicy,
			Storage:   jetstream.FileStorage,
			MaxAge:    24 * time.Hour, // safety cap
		},
		{
			// Dead-letter: keep dead runs for 30 days for inspection.
			Name:     StreamJobsDead,
			Subjects: []string{SubjectDead},
			Storage:  jetstream.FileStorage,
			MaxAge:   30 * 24 * time.Hour,
		},
		{
			// Event bus for live dashboard updates.
			// Memory storage is fine; events are ephemeral.
			Name:      StreamJobEvents,
			Subjects:  []string{SubjectEventAll},
			Storage:   jetstream.MemoryStorage,
			MaxAge:    1 * time.Hour,
			Retention: jetstream.InterestPolicy,
		},
	}

	for _, cfg := range streams {
		if _, err := c.JS.CreateOrUpdateStream(context.Background(), cfg); err != nil {
			return fmt.Errorf("ensure stream %s: %w", cfg.Name, err)
		}
	}
	return nil
}
