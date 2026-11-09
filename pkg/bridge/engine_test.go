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
