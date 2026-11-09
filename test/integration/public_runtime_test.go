//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/registry"
	bridgeruntime "gostreambridge/pkg/bridge/runtime"
)

func TestPublicRuntimeBuildsRegisteredPipeline(t *testing.T) {
	source := &runtimeSource{delivery: &runtimeDelivery{event: bridge.Event[[]byte]{
		Payload:  []byte(`{"order":"42"}`),
		Metadata: bridge.Metadata{ID: "event-42", Source: "integration"},
	}}}
	sink := &runtimeSink{}
	sourceName := uniqueName("public-runtime-source")
	sinkName := uniqueName("public-runtime-sink")

	registry.RegisterSource(sourceName, func(_ context.Context, config json.RawMessage, deps bridge.Dependencies) (bridge.Source[[]byte], error) {
		if string(config) != `{"consumer":"test"}` {
			t.Errorf("source config = %s", config)
		}
		if deps.Logger == nil || deps.Meter == nil || deps.Tracer == nil {
			t.Error("source factory did not receive default dependencies")
		}
		return source, nil
	})
	registry.RegisterSink(sinkName, func(_ context.Context, config json.RawMessage, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
		if string(config) != `{"producer":"test"}` {
			t.Errorf("sink config = %s", config)
		}
		if deps.Logger == nil || deps.Meter == nil || deps.Tracer == nil {
			t.Error("sink factory did not receive default dependencies")
		}
		return sink, nil
	})

	config, err := bridgeruntime.Load([]byte(`{
		"source": {"type": "` + sourceName + `", "config": {"consumer":"test"}},
		"sink": {"type": "` + sinkName + `", "config": {"producer":"test"}},
		"pipeline": {
			"workers": 1,
			"buffer": 0,
			"messageTimeout": 1000000000,
			"shutdownTimeout": 1000000000
		}
	}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pipeline, err := bridgeruntime.Build(context.Background(), config, bridge.Dependencies{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := pipeline.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if source.delivery.acks != 1 || source.delivery.nacks != 0 {
		t.Fatalf("delivery completion = ack %d, nack %d; want ack 1, nack 0", source.delivery.acks, source.delivery.nacks)
	}
	if !source.closed || !sink.closed {
		t.Fatalf("connector cleanup = source closed %t, sink closed %t; want both closed", source.closed, sink.closed)
	}
	if len(sink.events) != 1 || string(sink.events[0].Payload) != `{"order":"42"}` {
		t.Fatalf("sink events = %#v", sink.events)
	}
}

func TestPublicRegistryRejectsUnknownConnector(t *testing.T) {
	if _, err := bridgeruntime.Build(context.Background(), bridgeruntime.Config{
		Source:   bridgeruntime.Connector{Type: uniqueName("unknown-source")},
		Sink:     bridgeruntime.Connector{Type: uniqueName("unknown-sink")},
		Pipeline: bridge.PipelineConfig{Workers: 1, MessageTimeout: time.Second, ShutdownTimeout: time.Second},
	}, bridge.Dependencies{}); err == nil {
		t.Fatal("Build succeeded with an unregistered source")
	}
}

type runtimeDelivery struct {
	event       bridge.Event[[]byte]
	acks, nacks int
}

func (d *runtimeDelivery) Event() bridge.Event[[]byte]       { return d.event }
func (d *runtimeDelivery) Ack(context.Context) error         { d.acks++; return nil }
func (d *runtimeDelivery) Nack(context.Context, error) error { d.nacks++; return nil }

type runtimeSource struct {
	mu       sync.Mutex
	delivery *runtimeDelivery
	sent     bool
	closed   bool
}

func (s *runtimeSource) Open(context.Context) error { return nil }
func (s *runtimeSource) Receive(context.Context) (bridge.Delivery[[]byte], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sent {
		return nil, io.EOF
	}
	s.sent = true
	return s.delivery, nil
}
func (s *runtimeSource) Pause(context.Context) error  { return nil }
func (s *runtimeSource) Resume(context.Context) error { return nil }
func (s *runtimeSource) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type runtimeSink struct {
	mu     sync.Mutex
	events []bridge.Event[[]byte]
	closed bool
}

func (s *runtimeSink) Open(context.Context) error { return nil }
func (s *runtimeSink) Deliver(_ context.Context, event bridge.Event[[]byte]) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}
func (s *runtimeSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
