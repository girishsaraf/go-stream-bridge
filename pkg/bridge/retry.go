package bridge

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// ExponentialBackoff retries a bounded number of failed attempts with full
// jitter. Attempts includes the first delivery attempt.
type ExponentialBackoff struct {
	Attempts  int
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// Validate reports unsafe retry parameters.
func (p ExponentialBackoff) Validate() error {
	if p.Attempts <= 0 {
		return fmt.Errorf("attempts must be positive")
	}
	if p.BaseDelay <= 0 {
		return fmt.Errorf("base delay must be positive")
	}
	if p.MaxDelay < p.BaseDelay {
		return fmt.Errorf("max delay must be at least base delay")
	}
	return nil
}

// Next implements RetryPolicy.
func (p ExponentialBackoff) Next(attempt int, _ error) (time.Duration, bool) {
	if attempt >= p.Attempts {
		return 0, false
	}
	delay := p.BaseDelay
	for range attempt - 1 {
		if delay >= p.MaxDelay/2 {
			delay = p.MaxDelay
			break
		}
		delay *= 2
	}
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return time.Duration(rand.Int64N(int64(delay) + 1)), true
}
