package domain

import (
	"testing"
	"time"
)

func TestJobValidate(t *testing.T) {
	validPayload := map[string]any{
		"url":    "https://httpbin.org/post",
		"method": "POST",
	}
	validSchedule := Schedule{
		Kind:  ScheduleDelay,
		RunAt: func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
	}
	validPolicy := RetryPolicy{MaxRetries: 2, TimeoutSec: 30}

	tests := []struct {
		name    string
		job     Job
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid job",
			job: Job{
				Name: "Test Job", Type: "http_webhook",
				Payload: validPayload, Schedule: validSchedule, Policy: validPolicy,
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			job:     Job{Type: "http_webhook", Payload: validPayload, Schedule: validSchedule, Policy: validPolicy},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "invalid type",
			job:     Job{Name: "X", Type: "shell_exec", Payload: validPayload, Schedule: validSchedule, Policy: validPolicy},
			wantErr: true,
			errMsg:  "type must be http_webhook",
		},
		{
			name: "private IP blocked (SSRF)",
			job: Job{
				Name: "SSRF test", Type: "http_webhook",
				Payload:  map[string]any{"url": "http://192.168.1.1/internal"},
				Schedule: validSchedule, Policy: validPolicy,
			},
			wantErr: true,
			errMsg:  "private or loopback",
		},
		{
			name: "localhost blocked",
			job: Job{
				Name: "Localhost", Type: "http_webhook",
				Payload:  map[string]any{"url": "http://localhost/api"},
				Schedule: validSchedule, Policy: validPolicy,
			},
			wantErr: true,
		},
		{
			name: "timeout too low",
			job: Job{
				Name: "X", Type: "http_webhook", Payload: validPayload,
				Schedule: validSchedule,
				Policy:   RetryPolicy{MaxRetries: 1, TimeoutSec: 0},
			},
			wantErr: true,
			errMsg:  "timeout_sec",
		},
		{
			name: "retries too high",
			job: Job{
				Name: "X", Type: "http_webhook", Payload: validPayload,
				Schedule: validSchedule,
				Policy:   RetryPolicy{MaxRetries: 11, TimeoutSec: 30},
			},
			wantErr: true,
			errMsg:  "max_retries",
		},
		{
			name: "cron missing expr",
			job: Job{
				Name: "X", Type: "http_webhook", Payload: validPayload,
				Schedule: Schedule{Kind: ScheduleCron, Timezone: "UTC"},
				Policy:   validPolicy,
			},
			wantErr: true,
			errMsg:  "cron_expr",
		},
		{
			name: "cron missing timezone",
			job: Job{
				Name: "X", Type: "http_webhook", Payload: validPayload,
				Schedule: Schedule{Kind: ScheduleCron, CronExpr: "0 9 * * *"},
				Policy:   validPolicy,
			},
			wantErr: true,
			errMsg:  "timezone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidTransition(t *testing.T) {
	tests := []struct {
		from    RunState
		to      RunState
		wantErr bool
	}{
		{RunStateQueued, RunStateRunning, false},
		{RunStateRunning, RunStateSucceeded, false},
		{RunStateRunning, RunStateRetrying, false},
		{RunStateRunning, RunStateDead, false},
		{RunStateRetrying, RunStateRunning, false},
		// Invalid
		{RunStateSucceeded, RunStateRunning, true},
		{RunStateDead, RunStateRunning, true},
		{RunStateQueued, RunStateSucceeded, true},
	}
	for _, tt := range tests {
		err := ValidTransition(tt.from, tt.to)
		if tt.wantErr && err == nil {
			t.Errorf("%s -> %s: expected error", tt.from, tt.to)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("%s -> %s: unexpected error: %v", tt.from, tt.to, err)
		}
	}
}
