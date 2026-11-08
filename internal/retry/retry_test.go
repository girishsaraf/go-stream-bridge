package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	policy := Policy{Attempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}

	err := policy.Do(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	policy := Policy{Attempts: 5, BaseDelay: time.Hour, MaxDelay: time.Hour}

	attempts := 0
	err := policy.Do(ctx, func(context.Context) error {
		attempts++
		cancel()
		return errors.New("temporary failure")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoReturnsLastErrorAfterAttempts(t *testing.T) {
	lastErr := errors.New("still failing")
	policy := Policy{Attempts: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}

	err := policy.Do(context.Background(), func(context.Context) error {
		return lastErr
	})
	if !errors.Is(err, lastErr) {
		t.Fatalf("expected wrapped last error, got %v", err)
	}
}
