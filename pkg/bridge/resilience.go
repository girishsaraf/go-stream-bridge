package bridge

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Classification identifies whether an error should be retried.
type Classification uint8

const (
	Transient Classification = iota
	Permanent
	Throttled
	Cancelled
)

// ErrorClassifier categorizes connector failures.
type ErrorClassifier interface {
	Classify(error) Classification
}

// ErrorClassifierFunc adapts a function to ErrorClassifier.
type ErrorClassifierFunc func(error) Classification

// Classify invokes f.
func (f ErrorClassifierFunc) Classify(err error) Classification { return f(err) }

// DefaultClassifier recognizes context cancellation and treats all other
// errors as transient. Connectors should provide more precise classifiers.
func DefaultClassifier(err error) Classification {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Cancelled
	}
	return Transient
}

// DeadLetter contains a terminal delivery failure and its original event.
type DeadLetter[T any] struct {
	Event      Event[T]
	Error      string
	FailedAt   time.Time
	Attempts   int
	Classifier Classification
}

// DeadLetterSink persists terminal failures before their source delivery is
// acknowledged.
type DeadLetterSink[T any] interface {
	Sink[DeadLetter[T]]
}

// SinkFailureHandler converts terminal pipeline failures to dead-letter
// events. The caller must open and close the sink with the pipeline lifecycle.
type SinkFailureHandler[T any] struct {
	Sink       DeadLetterSink[T]
	Classifier ErrorClassifier
	Now        func() time.Time
}

// HandleFailure writes a canonical dead-letter record.
func (h SinkFailureHandler[T]) HandleFailure(ctx context.Context, event Event[T], err error) error {
	if h.Sink == nil {
		return errors.New("dead-letter sink is nil")
	}
	classifier := h.Classifier
	if classifier == nil {
		classifier = ErrorClassifierFunc(DefaultClassifier)
	}
	now := h.Now
	if now == nil {
		now = time.Now
	}
	return h.Sink.Deliver(ctx, Event[DeadLetter[T]]{
		Payload: DeadLetter[T]{
			Event:      event,
			Error:      err.Error(),
			FailedAt:   now().UTC(),
			Attempts:   event.Metadata.Attempt,
			Classifier: classifier.Classify(err),
		},
		Metadata: event.Metadata,
	})
}

// CircuitState is the current state of a CircuitBreaker.
type CircuitState uint8

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitOpenError is returned when a breaker rejects work.
type CircuitOpenError struct{}

func (CircuitOpenError) Error() string { return "circuit breaker is open" }

// CircuitBreaker executes calls while preventing cascades after repeated
// failures.
type CircuitBreaker interface {
	Execute(context.Context, func(context.Context) error) error
	State() CircuitState
}

// CircuitBreakerConfig controls a simple consecutive-failure breaker.
type CircuitBreakerConfig struct {
	FailureThreshold int
	OpenTimeout      time.Duration
	HalfOpenRequests int
}

// Validate reports unsafe circuit breaker settings.
func (c CircuitBreakerConfig) Validate() error {
	if c.FailureThreshold <= 0 {
		return errors.New("failure threshold must be positive")
	}
	if c.OpenTimeout <= 0 {
		return errors.New("open timeout must be positive")
	}
	if c.HalfOpenRequests <= 0 {
		return errors.New("half-open requests must be positive")
	}
	return nil
}

// ConsecutiveCircuitBreaker opens after consecutive classified failures.
type ConsecutiveCircuitBreaker struct {
	config     CircuitBreakerConfig
	classifier ErrorClassifier

	mu             sync.Mutex
	state          CircuitState
	failures       int
	openedAt       time.Time
	halfOpenActive int
}

// NewCircuitBreaker creates a breaker with the provided classifier.
func NewCircuitBreaker(config CircuitBreakerConfig, classifier ErrorClassifier) (*ConsecutiveCircuitBreaker, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid circuit breaker configuration: %w", err)
	}
	if classifier == nil {
		classifier = ErrorClassifierFunc(DefaultClassifier)
	}
	return &ConsecutiveCircuitBreaker{config: config, classifier: classifier}, nil
}

// Execute runs fn unless the breaker is open.
func (b *ConsecutiveCircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("circuit breaker function is nil")
	}
	if err := b.acquire(time.Now()); err != nil {
		return err
	}
	err := fn(ctx)
	b.release(err, time.Now())
	return err
}

// State returns the current state, transitioning an expired open breaker to
// half-open.
func (b *ConsecutiveCircuitBreaker) State() CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionExpiredOpen(time.Now())
	return b.state
}

func (b *ConsecutiveCircuitBreaker) acquire(now time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionExpiredOpen(now)
	switch b.state {
	case CircuitOpen:
		return CircuitOpenError{}
	case CircuitHalfOpen:
		if b.halfOpenActive >= b.config.HalfOpenRequests {
			return CircuitOpenError{}
		}
		b.halfOpenActive++
	}
	return nil
}

func (b *ConsecutiveCircuitBreaker) release(err error, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == CircuitHalfOpen {
		b.halfOpenActive--
	}
	if err == nil {
		b.failures = 0
		if b.state == CircuitHalfOpen {
			b.state = CircuitClosed
		}
		return
	}
	classification := b.classifier.Classify(err)
	if classification == Permanent || classification == Cancelled {
		return
	}
	b.failures++
	if b.state == CircuitHalfOpen || b.failures >= b.config.FailureThreshold {
		b.state = CircuitOpen
		b.openedAt = now
	}
}

func (b *ConsecutiveCircuitBreaker) transitionExpiredOpen(now time.Time) {
	if b.state == CircuitOpen && now.Sub(b.openedAt) >= b.config.OpenTimeout {
		b.state = CircuitHalfOpen
		b.halfOpenActive = 0
	}
}
