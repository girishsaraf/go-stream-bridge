package cdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
)

type config struct {
	Name string `json:"name"`
}

func (c config) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func TestDecodeConfigStrictlyDecodesAndValidates(t *testing.T) {
	got, err := cdk.DecodeConfig[config](json.RawMessage(`{"name":"orders"}`))
	if err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	if got.Name != "orders" {
		t.Fatalf("name = %q, want orders", got.Name)
	}

	for _, raw := range []json.RawMessage{
		nil,
		json.RawMessage(`null`),
		json.RawMessage(`{"name":"orders","unknown":true}`),
		json.RawMessage(`{"name":""}`),
		json.RawMessage(`{"name":"orders"} {}`),
	} {
		if _, err := cdk.DecodeConfig[config](raw); err == nil {
			t.Errorf("DecodeConfig(%s) succeeded", raw)
		}
	}
}

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

func TestConformanceHelpers(t *testing.T) {
	if _, ok := cdk.AssertSource[[]byte](testSource{}).(testSource); !ok {
		t.Fatal("AssertSource changed source")
	}
	if _, ok := cdk.AssertSink[[]byte](testSink{}).(testSink); !ok {
		t.Fatal("AssertSink changed sink")
	}
}

func TestFactoriesAreIsolatedAndApplyDefaults(t *testing.T) {
	sourceFactory := cdk.SourceFactory(func(_ context.Context, cfg config, deps bridge.Dependencies) (bridge.Source[[]byte], error) {
		if cfg.Name != "orders" {
			t.Errorf("config name = %q, want orders", cfg.Name)
		}
		if deps.Logger == nil || deps.Meter == nil || deps.Tracer == nil {
			t.Error("expected default dependencies")
		}
		return testSource{}, nil
	})
	sinkFactory := cdk.SinkFactory(func(_ context.Context, cfg config, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
		if cfg.Name != "orders" {
			t.Errorf("config name = %q, want orders", cfg.Name)
		}
		if deps.Logger == nil || deps.Meter == nil || deps.Tracer == nil {
			t.Error("expected default dependencies")
		}
		return testSink{}, nil
	})

	raw := json.RawMessage(`{"name":"orders"}`)
	if _, err := sourceFactory(context.Background(), raw, bridge.Dependencies{}); err != nil {
		t.Fatalf("source factory: %v", err)
	}
	if _, err := sinkFactory(context.Background(), raw, bridge.Dependencies{}); err != nil {
		t.Fatalf("sink factory: %v", err)
	}
}

func TestFactoriesRejectNilAndBuilderFailures(t *testing.T) {
	nilSource := cdk.SourceFactory[config](nil)
	if _, err := nilSource(context.Background(), json.RawMessage(`{"name":"orders"}`), bridge.Dependencies{}); err == nil {
		t.Fatal("nil source builder succeeded")
	}

	failingSink := cdk.SinkFactory(func(context.Context, config, bridge.Dependencies) (bridge.Sink[[]byte], error) {
		return nil, errors.New("unavailable")
	})
	_, err := failingSink(context.Background(), json.RawMessage(`{"name":"orders"}`), bridge.Dependencies{})
	if err == nil || !strings.Contains(err.Error(), "build sink") {
		t.Fatalf("sink factory error = %v, want wrapped build error", err)
	}
}
