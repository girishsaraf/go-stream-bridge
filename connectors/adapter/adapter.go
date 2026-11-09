// Package adapter adapts the legacy connector implementations to bridge contracts.
package adapter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"gostreambridge/internal/sink"
	"gostreambridge/internal/source"
	"gostreambridge/pkg/bridge"
)

var (
	// ErrResumeUnsupported reports that a paused legacy source cannot safely resume.
	ErrResumeUnsupported = errors.New("connector adapter: resume is not supported by the legacy source")
	// ErrDeliveryCompleted reports a duplicate acknowledgement attempt.
	ErrDeliveryCompleted = errors.New("connector adapter: delivery has already completed")
)

// Source adapts a legacy push-based source to the bridge pull-based source contract.
type Source struct {
	legacy source.Source

	mu     sync.Mutex
	cancel context.CancelFunc
	events chan bridge.Delivery[[]byte]
	done   chan struct{}
	err    error
	opened bool
	paused bool
	closed bool
}

// NewSource wraps a legacy source. The wrapped source is started by Open.
func NewSource(legacy source.Source) (*Source, error) {
	if legacy == nil {
		return nil, errors.New("legacy source is required")
	}
	return &Source{legacy: legacy}, nil
}

// Open starts consuming from the legacy source.
func (s *Source) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("connector adapter: source is closed")
	}
	if s.opened {
		return errors.New("connector adapter: source is already open")
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.events = make(chan bridge.Delivery[[]byte], 1)
	s.done = make(chan struct{})
	s.opened = true
	go s.consume(consumeCtx)
	return nil
}

func (s *Source) consume(ctx context.Context) {
	err := s.legacy.Consume(ctx, func(message source.Message) error {
		delivery := &delivery{message: message}
		select {
		case s.events <- delivery:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
	close(s.events)
	close(s.done)
}

// Receive returns the next legacy delivery or the caller's context error.
func (s *Source) Receive(ctx context.Context) (bridge.Delivery[[]byte], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	opened := s.opened
	events := s.events
	s.mu.Unlock()
	if !opened {
		return nil, errors.New("connector adapter: source is not open")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event, ok := <-events:
		if ok {
			return event, nil
		}
		s.mu.Lock()
		err := s.err
		s.mu.Unlock()
		if err == nil || errors.Is(err, context.Canceled) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("consume legacy source: %w", err)
	}
}

// Pause stops legacy consumption and waits for intake to finish. Deliveries
// already returned from Receive remain valid and can still be acknowledged.
func (s *Source) Pause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if !s.opened {
		s.mu.Unlock()
		return errors.New("connector adapter: source is not open")
	}
	if s.paused {
		done := s.done
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil
		}
	}
	s.paused = true
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()

	cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Resume reports that legacy sources do not support broker intake resumption.
func (*Source) Resume(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrResumeUnsupported
}

// Close cancels consumption and releases the wrapped source.
func (s *Source) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	done := make(chan error, 1)
	go func() { done <- s.legacy.Close() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// Sink adapts a legacy sink to the bridge sink contract.
type Sink struct {
	legacy sink.Sink
	mu     sync.Mutex
	opened bool
	closed bool
}

// NewSink wraps a legacy sink.
func NewSink(legacy sink.Sink) (*Sink, error) {
	if legacy == nil {
		return nil, errors.New("legacy sink is required")
	}
	return &Sink{legacy: legacy}, nil
}

// Open validates that the sink is available for use.
func (s *Sink) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("connector adapter: sink is closed")
	}
	if s.opened {
		return errors.New("connector adapter: sink is already open")
	}
	s.opened = true
	return nil
}

// Deliver writes the event payload using the legacy sink.
func (s *Sink) Deliver(ctx context.Context, event bridge.Event[[]byte]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	opened, closed := s.opened, s.closed
	s.mu.Unlock()
	if !opened || closed {
		return errors.New("connector adapter: sink is not open")
	}
	return s.legacy.Write(ctx, event.Payload)
}

// Close releases the wrapped sink.
func (s *Sink) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- s.legacy.Close() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

type delivery struct {
	message source.Message
	once    sync.Once
	err     error
}

func (d *delivery) Event() bridge.Event[[]byte] {
	return bridge.Event[[]byte]{
		Payload: d.message.Value,
		Metadata: bridge.Metadata{
			Source:    d.message.Source,
			Topic:     d.message.Topic,
			Partition: d.message.Partition,
			Offset:    d.message.Offset,
		},
	}
}

func (d *delivery) Ack(ctx context.Context) error {
	return d.complete(ctx, nil)
}

func (d *delivery) Nack(ctx context.Context, cause error) error {
	if cause == nil {
		cause = errors.New("delivery rejected")
	}
	return d.complete(ctx, cause)
}

func (d *delivery) complete(ctx context.Context, cause error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	called := false
	d.once.Do(func() {
		called = true
		if d.message.Ack != nil {
			d.err = d.message.Ack(cause)
		}
	})
	if !called {
		return ErrDeliveryCompleted
	}
	return d.err
}

var (
	_ bridge.Source[[]byte] = (*Source)(nil)
	_ bridge.Sink[[]byte]   = (*Sink)(nil)
)
