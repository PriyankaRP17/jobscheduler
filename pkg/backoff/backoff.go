package backoff

import (
	"math"
	"math/rand"
	"time"
)

// Config controls exponential backoff with jitter.
type Config struct {
	Base    time.Duration // initial delay (e.g. 2s)
	Cap     time.Duration // maximum delay (e.g. 5m)
	Jitter  float64       // fraction of delay added randomly (0.0-1.0)
}

// DefaultConfig is the production backoff used by workers.
var DefaultConfig = Config{
	Base:   2 * time.Second,
	Cap:    5 * time.Minute,
	Jitter: 0.25,
}

// Delay returns the delay for a given attempt number (1-based).
// Formula: min(Base * 2^(attempt-1), Cap) + random(0, delay*Jitter)
func (c Config) Delay(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(c.Base) * exp)
	if delay > c.Cap {
		delay = c.Cap
	}
	// Add jitter: random fraction of the delay
	jitter := time.Duration(float64(delay) * c.Jitter * rand.Float64())
	return delay + jitter
}

// Delay is a convenience function using DefaultConfig.
func Delay(attempt int) time.Duration {
	return DefaultConfig.Delay(attempt)
}
