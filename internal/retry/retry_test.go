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

func TestDoAbortsOnPermanentError(t *testing.T) {
	permanent := errors.New("permanent")
	policy := Policy{
		Attempts:  5,
		BaseDelay: time.Millisecond,
		MaxDelay:  time.Millisecond,
		Abort:     func(err error) bool { return errors.Is(err, permanent) },
	}

	attempts := 0
	err := policy.Do(context.Background(), func(context.Context) error {
		attempts++
		return permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("expected the original error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoReturnsContextErrorBeforeFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	err := Policy{Attempts: 3}.Do(ctx, func(context.Context) error {
		attempts++
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts != 0 {
		t.Fatalf("expected no attempts, got %d", attempts)
	}
}

func TestDoReportsRetryAttempts(t *testing.T) {
	var observed []int
	policy := Policy{
		Attempts:  3,
		BaseDelay: time.Millisecond,
		MaxDelay:  time.Millisecond,
		OnRetry:   func(attempt int, _ error) { observed = append(observed, attempt) },
	}

	err := policy.Do(context.Background(), func(context.Context) error {
		return errors.New("temporary")
	})
	if err == nil {
		t.Fatal("expected retries to return the last error")
	}

	if len(observed) != 2 || observed[0] != 1 || observed[1] != 2 {
		t.Fatalf("unexpected retry attempts: %v", observed)
	}
}
