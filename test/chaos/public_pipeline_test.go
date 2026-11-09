//go:build chaos

package chaos

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"gostreambridge/pkg/bridge"
)

func TestPipelineNacksSinkFailureAndClosesConnectors(t *testing.T) {
	delivery := &chaosDelivery{event: bridge.Event[[]byte]{Metadata: bridge.Metadata{ID: "sink-failure"}}}
	source := &chaosSource{delivery: delivery}
	sink := &chaosSink{err: errors.New("sink unavailable")}
	pipeline := newChaosPipeline(t, source, sink)

	err := pipeline.Run(context.Background())
	if err == nil {
		t.Fatal("Run succeeded after sink failure")
	}
	if delivery.acks != 0 || delivery.nacks != 1 {
		t.Fatalf("delivery completion = ack %d, nack %d; want ack 0, nack 1", delivery.acks, delivery.nacks)
	}
	if !source.closed || !sink.closed {
		t.Fatalf("connector cleanup = source closed %t, sink closed %t; want both closed", source.closed, sink.closed)
	}
}

func TestPipelineNacksCorruptPayloadAsSchemaFailure(t *testing.T) {
	delivery := &chaosDelivery{event: bridge.Event[[]byte]{
		Payload:  []byte(`{"id":`),
		Metadata: bridge.Metadata{ID: "corrupt-payload", Schema: bridge.SchemaReference{Subject: "orders", Format: "json"}},
	}}
	source := &chaosSource{delivery: delivery}
	sink := &chaosSink{err: errors.New("schema validation: malformed json")}
	pipeline := newChaosPipeline(t, source, sink)

	err := pipeline.Run(context.Background())
	if err == nil || !errors.Is(err, sink.err) {
		t.Fatalf("Run error = %v, want schema failure", err)
	}
	if delivery.acks != 0 || delivery.nacks != 1 {
		t.Fatalf("delivery completion = ack %d, nack %d; want ack 0, nack 1", delivery.acks, delivery.nacks)
	}
}

func TestPipelineCancellationReleasesBlockedSinkAndCleansUp(t *testing.T) {
	delivery := &chaosDelivery{event: bridge.Event[[]byte]{Metadata: bridge.Metadata{ID: "cancelled"}}}
	source := &chaosSource{delivery: delivery, waitAfterDelivery: true}
	sink := &chaosSink{started: make(chan struct{}), gate: make(chan struct{})}
	pipeline := newChaosPipeline(t, source, sink)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() { done <- pipeline.Run(ctx) }()
	waitClosed(t, sink.started, "sink delivery to start")
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}

	if delivery.acks != 0 || delivery.nacks != 0 {
		t.Fatalf("cancelled delivery completion = ack %d, nack %d; want neither", delivery.acks, delivery.nacks)
	}
	if !source.closed || !sink.closed {
		t.Fatalf("connector cleanup = source closed %t, sink closed %t; want both closed", source.closed, sink.closed)
	}
}

func newChaosPipeline(t *testing.T, source *chaosSource, sink *chaosSink) *bridge.Pipeline[[]byte] {
	t.Helper()
	pipeline, err := bridge.NewPipeline(source, sink, bridge.PipelineConfig{
		Workers: 1, Buffer: 0, MessageTimeout: time.Second, ShutdownTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	return pipeline
}

func waitClosed(t *testing.T, ch <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

type chaosDelivery struct {
	event       bridge.Event[[]byte]
	acks, nacks int
}

func (d *chaosDelivery) Event() bridge.Event[[]byte]       { return d.event }
func (d *chaosDelivery) Ack(context.Context) error         { d.acks++; return nil }
func (d *chaosDelivery) Nack(context.Context, error) error { d.nacks++; return nil }

type chaosSource struct {
	mu                sync.Mutex
	delivery          *chaosDelivery
	sent, closed      bool
	waitAfterDelivery bool
}

func (s *chaosSource) Open(context.Context) error { return nil }
func (s *chaosSource) Receive(ctx context.Context) (bridge.Delivery[[]byte], error) {
	s.mu.Lock()
	if !s.sent {
		s.sent = true
		s.mu.Unlock()
		return s.delivery, nil
	}
	wait := s.waitAfterDelivery
	s.mu.Unlock()
	if wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, io.EOF
}
func (s *chaosSource) Pause(context.Context) error  { return nil }
func (s *chaosSource) Resume(context.Context) error { return nil }
func (s *chaosSource) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type chaosSink struct {
	mu      sync.Mutex
	err     error
	started chan struct{}
	gate    <-chan struct{}
	closed  bool
}

func (s *chaosSink) Open(context.Context) error { return nil }
func (s *chaosSink) Deliver(ctx context.Context, _ bridge.Event[[]byte]) error {
	if s.started != nil {
		close(s.started)
	}
	if s.gate != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.gate:
		}
	}
	return s.err
}
func (s *chaosSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
