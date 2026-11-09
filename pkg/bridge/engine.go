package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// PipelineConfig bounds concurrent and queued unacknowledged deliveries.
type PipelineConfig struct {
	Workers         int
	Buffer          int
	MessageTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// Validate rejects configurations that cannot bound or drain a pipeline.
func (c PipelineConfig) Validate() error {
	if c.Workers <= 0 {
		return fmt.Errorf("workers must be positive")
	}
	if c.Buffer < 0 {
		return fmt.Errorf("buffer must not be negative")
	}
	if c.MessageTimeout <= 0 {
		return fmt.Errorf("message timeout must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	return nil
}

// FailureHandler durably handles a terminal sink failure, such as by writing a
// dead-letter record. Returning nil permits the original delivery to be
// acknowledged.
type FailureHandler[T any] interface {
	HandleFailure(context.Context, Event[T], error) error
}

// RetryPolicy decides whether a failed attempt should be retried and how long
// the engine should wait before the next attempt.
type RetryPolicy interface {
	Next(attempt int, err error) (delay time.Duration, retry bool)
}

// Pipeline is a bounded at-least-once source-to-sink runtime.
type Pipeline[T any] struct {
	source         Source[T]
	sink           Sink[T]
	failureHandler FailureHandler[T]
	retry          RetryPolicy
	classifier     ErrorClassifier
	breaker        CircuitBreaker
	config         PipelineConfig

	stateMu      sync.Mutex
	intakeCancel context.CancelFunc
	done         chan struct{}
	running      bool
}

// PipelineOption configures optional pipeline behavior.
type PipelineOption[T any] func(*Pipeline[T])

// WithRetry configures a retry policy. A nil policy disables retries.
func WithRetry[T any](policy RetryPolicy) PipelineOption[T] {
	return func(p *Pipeline[T]) {
		p.retry = policy
	}
}

// WithErrorClassifier controls which failures are eligible for retry.
func WithErrorClassifier[T any](classifier ErrorClassifier) PipelineOption[T] {
	return func(p *Pipeline[T]) {
		if classifier != nil {
			p.classifier = classifier
		}
	}
}

// WithCircuitBreaker applies a breaker around each sink delivery attempt.
func WithCircuitBreaker[T any](breaker CircuitBreaker) PipelineOption[T] {
	return func(p *Pipeline[T]) {
		p.breaker = breaker
	}
}

// NewPipeline creates a pipeline. Call Run once to start it.
func NewPipeline[T any](source Source[T], sink Sink[T], config PipelineConfig, failureHandler FailureHandler[T], options ...PipelineOption[T]) (*Pipeline[T], error) {
	if source == nil {
		return nil, errors.New("source is required")
	}
	if sink == nil {
		return nil, errors.New("sink is required")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pipeline configuration: %w", err)
	}
	pipeline := &Pipeline[T]{
		source:         source,
		sink:           sink,
		failureHandler: failureHandler,
		config:         config,
		classifier:     ErrorClassifierFunc(DefaultClassifier),
	}
	for _, option := range options {
		if option != nil {
			option(pipeline)
		}
	}
	return pipeline, nil
}

// Run opens connectors and processes events until the source finishes, a
// terminal error occurs, or Shutdown is called. io.EOF from Receive is a clean
// source completion.
func (p *Pipeline[T]) Run(ctx context.Context) (runErr error) {
	if err := p.start(); err != nil {
		return err
	}
	defer p.stop()

	if err := p.sink.Open(ctx); err != nil {
		return fmt.Errorf("open sink: %w", err)
	}
	sinkOpened := true
	defer func() {
		if sinkOpened {
			runErr = errors.Join(runErr, closeComponent(ctx, "close sink", p.sink.Close))
		}
	}()

	if err := p.source.Open(ctx); err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	sourceOpened := true
	defer func() {
		if sourceOpened {
			runErr = errors.Join(runErr, closeComponent(ctx, "close source", p.source.Close))
		}
	}()

	intakeCtx, intakeCancel := context.WithCancel(ctx)
	p.setIntakeCancel(intakeCancel)
	defer intakeCancel()

	jobs := make(chan Delivery[T], p.config.Buffer)
	var workers sync.WaitGroup
	errs := make(chan error, 1)
	for range p.config.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for delivery := range jobs {
				if err := p.process(ctx, delivery); err != nil {
					select {
					case errs <- err:
					default:
					}
					intakeCancel()
					return
				}
			}
		}()
	}

	receiveErr := p.receive(intakeCtx, jobs)
	close(jobs)
	workers.Wait()

	select {
	case workerErr := <-errs:
		return errors.Join(normalizeReceiveError(receiveErr), workerErr)
	default:
		return normalizeReceiveError(receiveErr)
	}
}

func (p *Pipeline[T]) receive(ctx context.Context, jobs chan<- Delivery[T]) error {
	for {
		delivery, err := p.source.Receive(ctx)
		if err != nil {
			return err
		}
		select {
		case jobs <- delivery:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Pipeline[T]) process(parent context.Context, delivery Delivery[T]) error {
	event := delivery.Event()
	messageCtx, cancel := context.WithTimeout(parent, p.config.MessageTimeout)
	defer cancel()

	err := p.deliver(messageCtx, event)
	switch {
	case err == nil:
		if ackErr := delivery.Ack(messageCtx); ackErr != nil {
			return fmt.Errorf("acknowledge event %q: %w", event.Metadata.ID, ackErr)
		}
		return nil
	case parent.Err() != nil:
		return nil
	case p.failureHandler != nil:
		if failureErr := p.failureHandler.HandleFailure(messageCtx, event, err); failureErr == nil {
			if ackErr := delivery.Ack(messageCtx); ackErr != nil {
				return fmt.Errorf("acknowledge dead-lettered event %q: %w", event.Metadata.ID, ackErr)
			}
			return nil
		} else {
			err = errors.Join(err, fmt.Errorf("handle terminal failure: %w", failureErr))
		}

		if nackErr := delivery.Nack(messageCtx, err); nackErr != nil {
			return errors.Join(err, fmt.Errorf("nacknowledge event %q: %w", event.Metadata.ID, nackErr))
		}
		return err
	default:
		if nackErr := delivery.Nack(messageCtx, err); nackErr != nil {
			return errors.Join(err, fmt.Errorf("nacknowledge event %q: %w", event.Metadata.ID, nackErr))
		}
		return fmt.Errorf("deliver event %q: %w", event.Metadata.ID, err)
	}
}

func (p *Pipeline[T]) deliver(ctx context.Context, event Event[T]) error {
	for attempt := 1; ; attempt++ {
		deliver := func(callCtx context.Context) error {
			return p.sink.Deliver(callCtx, event)
		}
		var err error
		if p.breaker != nil {
			err = p.breaker.Execute(ctx, deliver)
		} else {
			err = deliver(ctx)
		}
		if err == nil || p.retry == nil {
			return err
		}
		classification := p.classifier.Classify(err)
		if classification != Transient && classification != Throttled {
			return err
		}
		delay, retry := p.retry.Next(attempt, err)
		if !retry {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		}
	}
}

// Shutdown stops intake and waits for queued work to drain until ctx expires.
// Events still in progress at the deadline remain unacknowledged.
func (p *Pipeline[T]) Shutdown(ctx context.Context) error {
	p.stateMu.Lock()
	stopIntake := p.intakeCancel
	done := p.done
	running := p.running
	p.stateMu.Unlock()
	if !running {
		return nil
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, p.config.ShutdownTimeout)
	defer shutdownCancel()
	if err := p.source.Pause(shutdownCtx); err != nil {
		return fmt.Errorf("pause source: %w", err)
	}
	stopIntake()
	select {
	case <-done:
		return nil
	case <-shutdownCtx.Done():
		return fmt.Errorf("drain pipeline: %w", shutdownCtx.Err())
	}
}

func (p *Pipeline[T]) start() error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if p.running {
		return errors.New("pipeline is already running")
	}
	p.running = true
	p.done = make(chan struct{})
	return nil
}

func (p *Pipeline[T]) stop() {
	p.stateMu.Lock()
	p.running = false
	p.intakeCancel = nil
	close(p.done)
	p.stateMu.Unlock()
}

func (p *Pipeline[T]) setIntakeCancel(cancel context.CancelFunc) {
	p.stateMu.Lock()
	p.intakeCancel = cancel
	p.stateMu.Unlock()
}

func closeComponent(ctx context.Context, name string, close func(context.Context) error) error {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	if err := close(closeCtx); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func normalizeReceiveError(err error) error {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return fmt.Errorf("receive event: %w", err)
}

var _ Engine[[]byte] = (*Pipeline[[]byte])(nil)
