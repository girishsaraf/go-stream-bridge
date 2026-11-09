package registry

import (
	"context"
	"encoding/json"
	"testing"

	"gostreambridge/pkg/bridge"
)

type testSource struct{}

func (testSource) Open(context.Context) error { return nil }
func (testSource) Receive(context.Context) (bridge.Delivery[[]byte], error) {
	return nil, context.Canceled
}
func (testSource) Pause(context.Context) error  { return nil }
func (testSource) Resume(context.Context) error { return nil }
func (testSource) Close(context.Context) error  { return nil }

type testSink struct{}

func (testSink) Open(context.Context) error                          { return nil }
func (testSink) Deliver(context.Context, bridge.Event[[]byte]) error { return nil }
func (testSink) Close(context.Context) error                         { return nil }

func TestFactoriesNormalizeNamesAndInjectDefaults(t *testing.T) {
	const sourceName = "registry-test-source"
	const sinkName = "registry-test-sink"
	RegisterSource(sourceName, func(_ context.Context, _ json.RawMessage, deps bridge.Dependencies) (bridge.Source[[]byte], error) {
		if deps.Logger == nil || deps.Meter == nil || deps.Tracer == nil {
			t.Fatal("expected default dependencies")
		}
		return testSource{}, nil
	})
	RegisterSink(sinkName, func(_ context.Context, _ json.RawMessage, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
		if deps.Logger == nil || deps.Meter == nil || deps.Tracer == nil {
			t.Fatal("expected default dependencies")
		}
		return testSink{}, nil
	})

	if _, err := NewSource(context.Background(), " REGISTRY-TEST-SOURCE ", nil, bridge.Dependencies{}); err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := NewSink(context.Background(), " REGISTRY-TEST-SINK ", nil, bridge.Dependencies{}); err != nil {
		t.Fatalf("NewSink: %v", err)
	}
}

func TestUnknownConnectorReturnsError(t *testing.T) {
	if _, err := NewSource(context.Background(), "missing", nil, bridge.Dependencies{}); err == nil {
		t.Fatal("NewSource succeeded for an unknown connector")
	}
	if _, err := NewSink(context.Background(), "missing", nil, bridge.Dependencies{}); err == nil {
		t.Fatal("NewSink succeeded for an unknown connector")
	}
}
