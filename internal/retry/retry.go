// Package retry provides a small context-aware retry helper shared by sinks.
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
}

// Default returns the standard policy used by downstream sinks.
func Default() Policy {
	return Policy{
		Attempts:  5,
		BaseDelay: 2 * time.Second,
		MaxDelay:  10 * time.Second,
	}
}

// Do runs fn until it succeeds, the attempt limit is reached, or ctx is cancelled.
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

	return fmt.Errorf("retry after %d attempts: %w", attempts, lastErr)
}
