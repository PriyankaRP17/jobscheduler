package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// JobStatus controls whether the scheduler will enqueue this job.
type JobStatus string

const (
	JobStatusActive    JobStatus = "active"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed" // once/delay jobs after firing
)

// ScheduleKind is the type of schedule.
type ScheduleKind string

const (
	ScheduleOnce  ScheduleKind = "once"
	ScheduleDelay ScheduleKind = "delay"
	ScheduleCron  ScheduleKind = "cron"
)

// Schedule holds the job's timing definition.
type Schedule struct {
	Kind     ScheduleKind `bson:"kind" json:"kind"`
	CronExpr string       `bson:"cron_expr,omitempty" json:"cron_expr,omitempty"`
	Timezone string       `bson:"timezone,omitempty" json:"timezone,omitempty"`
	RunAt    *time.Time   `bson:"run_at,omitempty" json:"run_at,omitempty"` // once / delay
}

// RetryPolicy controls retry behaviour.
type RetryPolicy struct {
	MaxRetries int    `bson:"max_retries" json:"max_retries"` // 0 = no retries
	TimeoutSec int    `bson:"timeout_sec" json:"timeout_sec"` // 1-300
	Priority   string `bson:"priority" json:"priority"`       // "high" | "default"
}

// Job is the core entity: a definition of work plus its schedule.
type Job struct {
	ID        string            `bson:"_id" json:"id"`
	OwnerID   string            `bson:"owner_id" json:"owner_id"`
	Name      string            `bson:"name" json:"name"`
	Type      string            `bson:"type" json:"type"` // "http_webhook"
	Payload   map[string]any    `bson:"payload" json:"payload"`
	Schedule  Schedule          `bson:"schedule" json:"schedule"`
	Policy    RetryPolicy       `bson:"policy" json:"policy"`
	Status    JobStatus         `bson:"status" json:"status"`
	NextRunAt *time.Time        `bson:"next_run_at,omitempty" json:"next_run_at,omitempty"`
	CreatedAt time.Time         `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time         `bson:"updated_at" json:"updated_at"`
}

// Validate checks the job fields before saving.
func (j *Job) Validate() error {
	if strings.TrimSpace(j.Name) == "" {
		return errors.New("name is required")
	}
	if len(j.Name) > 100 {
		return errors.New("name must be 100 characters or fewer")
	}
	if j.Type != "http_webhook" {
		return errors.New("type must be http_webhook")
	}
	if err := validateWebhookPayload(j.Payload); err != nil {
		return err
	}
	if j.Policy.TimeoutSec < 1 || j.Policy.TimeoutSec > 300 {
		return errors.New("timeout_sec must be between 1 and 300")
	}
	if j.Policy.MaxRetries < 0 || j.Policy.MaxRetries > 10 {
		return errors.New("max_retries must be between 0 and 10")
	}
	if j.Policy.Priority == "" {
		j.Policy.Priority = "default"
	}
	if j.Policy.Priority != "high" && j.Policy.Priority != "default" {
		return errors.New("priority must be high or default")
	}
	return validateSchedule(j.Schedule)
}

func validateWebhookPayload(p map[string]any) error {
	rawURL, ok := p["url"].(string)
	if !ok || rawURL == "" {
		return errors.New("payload.url is required")
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.New("payload.url is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("payload.url must use http or https")
	}
	// SSRF guard is applied again at execution time (DNS can change).
	if isPrivateHost(u.Hostname()) {
		return errors.New("payload.url must not point to a private or loopback address")
	}
	return nil
}

func validateSchedule(s Schedule) error {
	switch s.Kind {
	case ScheduleOnce:
		if s.RunAt == nil {
			return errors.New("schedule.run_at is required for kind=once")
		}
	case ScheduleDelay:
		if s.RunAt == nil {
			return errors.New("schedule.run_at is required for kind=delay")
		}
	case ScheduleCron:
		if strings.TrimSpace(s.CronExpr) == "" {
			return errors.New("schedule.cron_expr is required for kind=cron")
		}
		if s.Timezone == "" {
			return errors.New("schedule.timezone is required for kind=cron")
		}
	default:
		return errors.New("schedule.kind must be once, delay, or cron")
	}
	return nil
}

// isPrivateHost blocks loopback, private and link-local ranges.
// Full IP resolution is done again at execution time.
func isPrivateHost(host string) bool {
	lower := strings.ToLower(host)
	blocked := []string{
		"localhost", "127.", "0.0.0.0",
		"10.", "172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.", "172.24.",
		"172.25.", "172.26.", "172.27.", "172.28.", "172.29.",
		"172.30.", "172.31.", "192.168.",
		"169.254.", // link-local / cloud metadata
		"::1", "fc", "fd",
	}
	for _, prefix := range blocked {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
