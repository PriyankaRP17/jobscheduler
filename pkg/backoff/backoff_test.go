package backoff

import (
	"testing"
	"time"
)

func TestDelay_Increases(t *testing.T) {
	cfg := Config{Base: 2 * time.Second, Cap: 5 * time.Minute, Jitter: 0}

	prev := time.Duration(0)
	for attempt := 1; attempt <= 6; attempt++ {
		d := cfg.Delay(attempt)
		if d <= prev {
			t.Errorf("attempt %d: delay %v did not increase from %v", attempt, d, prev)
		}
		prev = d
	}
}

func TestDelay_Cap(t *testing.T) {
	cfg := Config{Base: 2 * time.Second, Cap: 10 * time.Second, Jitter: 0}
	// Attempt 4 would be 2^3 * 2s = 16s, but cap is 10s.
	for attempt := 4; attempt <= 10; attempt++ {
		d := cfg.Delay(attempt)
		if d > cfg.Cap+cfg.Cap/4 { // small tolerance for 0-jitter
			t.Errorf("attempt %d: delay %v exceeds cap %v", attempt, d, cfg.Cap)
		}
	}
}

func TestDelay_Jitter(t *testing.T) {
	cfg := Config{Base: 2 * time.Second, Cap: 5 * time.Minute, Jitter: 0.25}
	// With jitter, two calls for the same attempt should sometimes differ.
	seen := make(map[time.Duration]bool)
	for i := 0; i < 20; i++ {
		seen[cfg.Delay(1)] = true
	}
	if len(seen) == 1 {
		t.Error("jitter produced the same value 20 times in a row")
	}
}
