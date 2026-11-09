package bridge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	breaker, err := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenTimeout:      time.Millisecond,
		HalfOpenRequests: 1,
	}, ErrorClassifierFunc(func(error) Classification { return Transient }))
	if err != nil {
		t.Fatalf("NewCircuitBreaker: %v", err)
	}
	if err := breaker.Execute(context.Background(), func(context.Context) error { return errors.New("down") }); err == nil {
		t.Fatal("Execute succeeded")
	}
	if err := breaker.Execute(context.Background(), func(context.Context) error { return nil }); !errors.As(err, new(CircuitOpenError)) {
		t.Fatalf("Execute error = %v, want CircuitOpenError", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := breaker.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("half-open Execute: %v", err)
	}
	if state := breaker.State(); state != CircuitClosed {
		t.Fatalf("state = %v, want closed", state)
	}
}
