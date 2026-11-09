package bridge

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type memoryDelivery struct {
	event         Event[string]
	acked, nacked int
}

func (d *memoryDelivery) Event() Event[string]              { return d.event }
func (d *memoryDelivery) Ack(context.Context) error         { d.acked++; return nil }
func (d *memoryDelivery) Nack(context.Context, error) error { d.nacked++; return nil }

type memorySource struct {
	deliveries []*memoryDelivery
	mu         sync.Mutex
	index      int
	paused     bool
}

func (s *memorySource) Open(context.Context) error  { return nil }
func (s *memorySource) Close(context.Context) error { return nil }
func (s *memorySource) Pause(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
	return nil
}
func (s *memorySource) Resume(context.Context) error { return nil }
func (s *memorySource) Receive(context.Context) (Delivery[string], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index == len(s.deliveries) {
		return nil, io.EOF
	}
	delivery := s.deliveries[s.index]
	s.index++
	return delivery, nil
}

type memorySink struct {
	err error
}

func (s memorySink) Open(context.Context) error                   { return nil }
func (s memorySink) Close(context.Context) error                  { return nil }
func (s memorySink) Deliver(context.Context, Event[string]) error { return s.err }

func testPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Workers: 2, Buffer: 2, MessageTimeout: time.Second, ShutdownTimeout: time.Second,
	}
}

func TestPipelineAcknowledgesSuccessfulDeliveries(t *testing.T) {
	delivery := &memoryDelivery{event: Event[string]{Metadata: Metadata{ID: "one"}}}
	pipeline, err := NewPipeline[string](&memorySource{deliveries: []*memoryDelivery{delivery}}, memorySink{}, testPipelineConfig(), nil)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	if err := pipeline.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if delivery.acked != 1 || delivery.nacked != 0 {
		t.Fatalf("ack=%d nack=%d, want ack=1 nack=0", delivery.acked, delivery.nacked)
	}
}

func TestPipelineNacksFailedDelivery(t *testing.T) {
	delivery := &memoryDelivery{event: Event[string]{Metadata: Metadata{ID: "one"}}}
	pipeline, err := NewPipeline[string](
		&memorySource{deliveries: []*memoryDelivery{delivery}},
		memorySink{err: errors.New("sink unavailable")},
		testPipelineConfig(),
		nil,
	)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	if err := pipeline.Run(context.Background()); err == nil {
		t.Fatal("Run succeeded")
	}
	if delivery.acked != 0 || delivery.nacked != 1 {
		t.Fatalf("ack=%d nack=%d, want ack=0 nack=1", delivery.acked, delivery.nacked)
	}
}

type retryingSink struct{ attempts int }

func (s *retryingSink) Open(context.Context) error  { return nil }
func (s *retryingSink) Close(context.Context) error { return nil }
func (s *retryingSink) Deliver(context.Context, Event[string]) error {
	s.attempts++
	if s.attempts < 3 {
		return errors.New("temporary")
	}
	return nil
}

func TestPipelineRetriesBeforeAcknowledging(t *testing.T) {
	delivery := &memoryDelivery{event: Event[string]{Metadata: Metadata{ID: "one"}}}
	sink := &retryingSink{}
	pipeline, err := NewPipeline(
		&memorySource{deliveries: []*memoryDelivery{delivery}},
		sink,
		testPipelineConfig(),
		nil,
		WithRetry[string](ExponentialBackoff{Attempts: 3, BaseDelay: time.Nanosecond, MaxDelay: time.Nanosecond}),
	)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	if err := pipeline.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sink.attempts != 3 {
		t.Fatalf("attempts = %d, want 3", sink.attempts)
	}
	if delivery.acked != 1 {
		t.Fatalf("ack = %d, want 1", delivery.acked)
	}
}

type recordingFailureHandler struct {
	called bool
	err    error
}

func (h *recordingFailureHandler) HandleFailure(context.Context, Event[string], error) error {
	h.called = true
	return h.err
}

func TestPipelineAcknowledgesAfterTerminalFailureHandler(t *testing.T) {
	delivery := &memoryDelivery{event: Event[string]{Metadata: Metadata{ID: "one"}}}
	handler := &recordingFailureHandler{}
	pipeline, err := NewPipeline(
		&memorySource{deliveries: []*memoryDelivery{delivery}},
		memorySink{err: errors.New("invalid payload")},
		testPipelineConfig(),
		handler,
	)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	if err := pipeline.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !handler.called || delivery.acked != 1 || delivery.nacked != 0 {
		t.Fatalf("handler=%v ack=%d nack=%d", handler.called, delivery.acked, delivery.nacked)
	}
}

type countingRetryPolicy struct{ calls int }

func (p *countingRetryPolicy) Next(int, error) (time.Duration, bool) {
	p.calls++
	return 0, true
}

func TestPipelineDoesNotRetryPermanentErrors(t *testing.T) {
	delivery := &memoryDelivery{event: Event[string]{Metadata: Metadata{ID: "one"}}}
	retry := &countingRetryPolicy{}
	pipeline, err := NewPipeline(
		&memorySource{deliveries: []*memoryDelivery{delivery}},
		memorySink{err: errors.New("schema violation")},
		testPipelineConfig(),
		nil,
		WithRetry[string](retry),
		WithErrorClassifier[string](ErrorClassifierFunc(func(error) Classification { return Permanent })),
	)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	if err := pipeline.Run(context.Background()); err == nil {
		t.Fatal("Run succeeded")
	}
	if retry.calls != 0 {
		t.Fatalf("retry calls = %d, want 0", retry.calls)
	}
}

type blockingSink struct {
	started chan struct{}
	release <-chan struct{}
}

func (s blockingSink) Open(context.Context) error  { return nil }
func (s blockingSink) Close(context.Context) error { return nil }
func (s blockingSink) Deliver(ctx context.Context, _ Event[string]) error {
	select {
	case s.started <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type trackingSource struct {
	deliveries []*memoryDelivery
	received   chan int
	blockAfter bool
	mu         sync.Mutex
	index      int
}

func (*trackingSource) Open(context.Context) error   { return nil }
func (*trackingSource) Close(context.Context) error  { return nil }
func (*trackingSource) Pause(context.Context) error  { return nil }
func (*trackingSource) Resume(context.Context) error { return nil }
func (s *trackingSource) Receive(ctx context.Context) (Delivery[string], error) {
	s.mu.Lock()
	if s.index < len(s.deliveries) {
		delivery := s.deliveries[s.index]
		s.index++
		index := s.index
		s.mu.Unlock()
		s.received <- index
		return delivery, nil
	}
	s.mu.Unlock()
	if !s.blockAfter {
		return nil, io.EOF
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPipelineAppliesBackpressureWhenWorkersAreBusy(t *testing.T) {
	deliveries := []*memoryDelivery{
		{event: Event[string]{Metadata: Metadata{ID: "one"}}},
		{event: Event[string]{Metadata: Metadata{ID: "two"}}},
		{event: Event[string]{Metadata: Metadata{ID: "three"}}},
	}
	source := &trackingSource{deliveries: deliveries, received: make(chan int, len(deliveries))}
	release := make(chan struct{})
	sink := blockingSink{started: make(chan struct{}, 1), release: release}
	config := testPipelineConfig()
	config.Workers, config.Buffer = 1, 0
	pipeline, err := NewPipeline(source, sink, config, nil)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- pipeline.Run(context.Background()) }()
	<-sink.started
	<-source.received
	<-source.received
	select {
	case received := <-source.received:
		t.Fatalf("received delivery %d while worker was blocked", received)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestPipelineShutdownDrainsInFlightDelivery(t *testing.T) {
	delivery := &memoryDelivery{event: Event[string]{Metadata: Metadata{ID: "one"}}}
	source := &trackingSource{deliveries: []*memoryDelivery{delivery}, received: make(chan int, 1), blockAfter: true}
	release := make(chan struct{})
	sink := blockingSink{started: make(chan struct{}, 1), release: release}
	pipeline, err := NewPipeline(source, sink, testPipelineConfig(), nil)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- pipeline.Run(context.Background()) }()
	<-sink.started
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- pipeline.Shutdown(context.Background()) }()
	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown returned before drain: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-shutdownDone; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if delivery.acked != 1 {
		t.Fatalf("ack = %d, want 1", delivery.acked)
	}
}
