// Package retry provides a small context-aware retry helper shared by sources,
// sinks, and the stream pipeline.
package retry

import (
	"context"
	"fmt"
	"time"
)

// Policy controls retry attempts and exponential backoff.
type Policy struct {
	Attempts  int
	BaseDelay time.Duration
	MaxDelay  time.Duration
	// OnRetry is called after a failed attempt and before the next one.
	OnRetry func(attempt int, err error)
	// Abort, when non-nil, stops retrying immediately when it returns true.
	// The original error is returned unwrapped.
	Abort func(error) bool
}

// Default returns the standard retry policy used by the pipeline.
func Default() Policy {
	return Policy{
		Attempts:  5,
		BaseDelay: 2 * time.Second,
		MaxDelay:  30 * time.Second,
	}
}

// Do runs fn until it succeeds, the attempt limit is reached, ctx is
// cancelled, or Abort reports a permanent error.
func (p Policy) Do(ctx context.Context, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	attempts := p.Attempts
	if attempts < 1 {
		attempts = 1
	}

	baseDelay := p.BaseDelay
	if baseDelay <= 0 {
		baseDelay = time.Second
	}

	maxDelay := p.MaxDelay
	if maxDelay < baseDelay {
		maxDelay = baseDelay
	}

	delay := baseDelay
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if p.Abort != nil && p.Abort(err) {
			return err
		}

		if attempt == attempts {
			break
		}

		if p.OnRetry != nil {
			p.OnRetry(attempt, lastErr)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", attempts, lastErr)
}
