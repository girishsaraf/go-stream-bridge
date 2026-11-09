// Package testkit provides deterministic bridge contract fakes for tests.
package testkit

import (
	"context"
	"errors"
	"io"
	"sync"

	"gostreambridge/pkg/bridge"
)

var (
	// ErrAlreadyCompleted is returned when a fake delivery is completed twice.
	ErrAlreadyCompleted = errors.New("testkit: delivery already completed")
	// ErrSourceClosed is returned when an event is added after Finish.
	ErrSourceClosed = errors.New("testkit: source is closed")
)

// Delivery is an inspectable bridge delivery. It is safe to inspect while the
// code under test is running.
type Delivery[T any] struct {
	event bridge.Event[T]

	mu          sync.Mutex
	ackCount    int
	nackCount   int
	nackErr     error
	completed   bool
	ackErr      error
	nackCallErr error
}

// NewDelivery constructs a delivery for event.
func NewDelivery[T any](event bridge.Event[T]) *Delivery[T] {
	return &Delivery[T]{event: cloneEvent(event)}
}

// Event returns the immutable event supplied at construction.
func (d *Delivery[T]) Event() bridge.Event[T] { return cloneEvent(d.event) }

// Ack marks the delivery successful. A delivery can only be terminally
// completed once, mirroring the bridge contract.
func (d *Delivery[T]) Ack(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.completed {
		return ErrAlreadyCompleted
	}
	d.ackCount++
	if d.ackErr != nil {
		return d.ackErr
	}
	d.completed = true
	return nil
}

// Nack marks the delivery unsuccessful and retains the supplied cause.
func (d *Delivery[T]) Nack(ctx context.Context, cause error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.completed {
		return ErrAlreadyCompleted
	}
	d.nackCount++
	d.nackErr = cause
	if d.nackCallErr != nil {
		return d.nackCallErr
	}
	d.completed = true
	return nil
}

// SetAckError makes future Ack calls return err without completing delivery.
func (d *Delivery[T]) SetAckError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ackErr = err
}

// SetNackError makes future Nack calls return err without completing delivery.
func (d *Delivery[T]) SetNackError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nackCallErr = err
}

// AckCount reports the number of Ack attempts.
func (d *Delivery[T]) AckCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ackCount
}

// NackCount reports the number of Nack attempts.
func (d *Delivery[T]) NackCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.nackCount
}

// NackCause reports the cause from the most recent Nack attempt.
func (d *Delivery[T]) NackCause() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.nackErr
}

// Completed reports whether Ack or Nack has completed the delivery.
func (d *Delivery[T]) Completed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.completed
}

// Source is a context-aware, controllable finite or streaming source.
// NewSource creates a finite source; NewOpenSource remains idle until an
// event is added or Finish is called.
type Source[T any] struct {
	mu sync.Mutex

	deliveries    []*Delivery[T]
	allDeliveries []*Delivery[T]
	receiveErrs   []error
	paused        bool
	finished      bool
	openErr       error
	closeErr      error
	opened        bool
	closed        bool
	changed       chan struct{}
}

// NewSource creates a finite source that returns io.EOF after events.
func NewSource[T any](events ...bridge.Event[T]) *Source[T] {
	source := NewOpenSource[T]()
	for _, event := range events {
		if _, err := source.Send(event); err != nil {
			panic(err)
		}
	}
	source.Finish()
	return source
}

// NewOpenSource creates a source that waits for events, Finish, or cancellation.
func NewOpenSource[T any]() *Source[T] {
	return &Source[T]{changed: make(chan struct{})}
}

// Open marks the source open.
func (s *Source[T]) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = true
	return s.openErr
}

// Receive returns the next delivery. While paused or empty it waits without
// polling and always unblocks when ctx is cancelled.
func (s *Source[T]) Receive(ctx context.Context) (bridge.Delivery[T], error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, io.EOF
		}
		if !s.paused && len(s.receiveErrs) > 0 {
			err := s.receiveErrs[0]
			s.receiveErrs = s.receiveErrs[1:]
			s.mu.Unlock()
			return nil, err
		}
		if !s.paused && len(s.deliveries) > 0 {
			delivery := s.deliveries[0]
			s.deliveries = s.deliveries[1:]
			s.mu.Unlock()
			return delivery, nil
		}
		if s.finished && len(s.deliveries) == 0 && len(s.receiveErrs) == 0 {
			s.mu.Unlock()
			return nil, io.EOF
		}
		changed := s.changed
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

// Pause stops Receive from releasing queued deliveries.
func (s *Source[T]) Pause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
	s.signalLocked()
	return nil
}

// Resume allows Receive to release queued deliveries.
func (s *Source[T]) Resume(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = false
	s.signalLocked()
	return nil
}

// Close ends Receive and unblocks waiters.
func (s *Source[T]) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.signalLocked()
	return s.closeErr
}

// Send queues event and returns its inspectable delivery.
func (s *Source[T]) Send(event bridge.Event[T]) (*Delivery[T], error) {
	delivery := NewDelivery(event)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.closed {
		return nil, ErrSourceClosed
	}
	s.deliveries = append(s.deliveries, delivery)
	s.allDeliveries = append(s.allDeliveries, delivery)
	s.signalLocked()
	return delivery, nil
}

// FailNext makes the next unpaused Receive return err. Multiple calls retain
// their call order.
func (s *Source[T]) FailNext(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receiveErrs = append(s.receiveErrs, err)
	s.signalLocked()
}

// Finish makes an empty source return io.EOF after previously queued items.
func (s *Source[T]) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = true
	s.signalLocked()
}

// SetOpenError configures Open to return err.
func (s *Source[T]) SetOpenError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openErr = err
}

// SetCloseError configures Close to return err.
func (s *Source[T]) SetCloseError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeErr = err
}

// Deliveries returns a snapshot of all deliveries created by Send.
func (s *Source[T]) Deliveries() []*Delivery[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Delivery[T](nil), s.allDeliveries...)
}

func (s *Source[T]) signalLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// Sink is an inspectable, context-aware sink. A delivery gate lets tests
// deterministically create downstream backpressure.
type Sink[T any] struct {
	mu sync.Mutex

	events         []bridge.Event[T]
	deliverErrs    []error
	gate           <-chan struct{}
	openErr        error
	closeErr       error
	opened, closed bool
}

// NewSink constructs a sink that succeeds unless configured otherwise.
func NewSink[T any]() *Sink[T] {
	return &Sink[T]{}
}

// Open marks the sink open.
func (s *Sink[T]) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = true
	return s.openErr
}

// Deliver records event, waits for an optional gate, and returns a scripted
// error in FIFO order. A blocked gate always respects ctx cancellation.
func (s *Sink[T]) Deliver(ctx context.Context, event bridge.Event[T]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.events = append(s.events, cloneEvent(event))
	gate := s.gate
	s.mu.Unlock()

	if gate != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-gate:
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.deliverErrs) == 0 {
		return nil
	}
	err := s.deliverErrs[0]
	s.deliverErrs = s.deliverErrs[1:]
	return err
}

// Close marks the sink closed.
func (s *Sink[T]) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.closeErr
}

// SetGate makes Deliver wait for a receive from gate. Set it to nil to disable
// blocking; closing a gate releases all waiting deliveries.
func (s *Sink[T]) SetGate(gate <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gate = gate
}

// FailNext makes the next completed Deliver return err.
func (s *Sink[T]) FailNext(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliverErrs = append(s.deliverErrs, err)
}

// SetOpenError configures Open to return err.
func (s *Sink[T]) SetOpenError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openErr = err
}

// SetCloseError configures Close to return err.
func (s *Sink[T]) SetCloseError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeErr = err
}

// Events returns a snapshot of delivery attempts in call order.
func (s *Sink[T]) Events() []bridge.Event[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]bridge.Event[T], len(s.events))
	for i, event := range s.events {
		events[i] = cloneEvent(event)
	}
	return events
}

// DeliveryCount reports all Deliver calls, including ones that returned errors.
func (s *Sink[T]) DeliveryCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func cloneEvent[T any](event bridge.Event[T]) bridge.Event[T] {
	event.Metadata.Key = append([]byte(nil), event.Metadata.Key...)
	if event.Metadata.Headers == nil {
		return event
	}
	headers := make(map[string][]byte, len(event.Metadata.Headers))
	for key, value := range event.Metadata.Headers {
		headers[key] = append([]byte(nil), value...)
	}
	event.Metadata.Headers = headers
	return event
}
