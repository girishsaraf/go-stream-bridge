package bridge_test

import (
	"context"
	"fmt"
	"io"
	"time"

	"gostreambridge/pkg/bridge"
)

func ExamplePipeline_Run() {
	source := &exampleSource{delivery: exampleDelivery{
		event: bridge.Event[string]{Metadata: bridge.Metadata{ID: "order-1"}, Payload: "created"},
	}}
	sink := exampleSink{}
	pipeline, err := bridge.NewPipeline(source, sink, bridge.PipelineConfig{
		Workers: 1, Buffer: 0, MessageTimeout: time.Second, ShutdownTimeout: time.Second,
	}, nil)
	if err != nil {
		panic(err)
	}
	if err := pipeline.Run(context.Background()); err != nil {
		panic(err)
	}

	// Output:
	// processed order-1: created
}

type exampleDelivery struct{ event bridge.Event[string] }

func (d exampleDelivery) Event() bridge.Event[string]     { return d.event }
func (exampleDelivery) Ack(context.Context) error         { return nil }
func (exampleDelivery) Nack(context.Context, error) error { return nil }

type exampleSource struct {
	delivery exampleDelivery
	received bool
}

func (*exampleSource) Open(context.Context) error   { return nil }
func (*exampleSource) Pause(context.Context) error  { return nil }
func (*exampleSource) Resume(context.Context) error { return nil }
func (*exampleSource) Close(context.Context) error  { return nil }
func (s *exampleSource) Receive(context.Context) (bridge.Delivery[string], error) {
	if s.received {
		return nil, io.EOF
	}
	s.received = true
	return s.delivery, nil
}

type exampleSink struct{}

func (exampleSink) Open(context.Context) error  { return nil }
func (exampleSink) Close(context.Context) error { return nil }
func (exampleSink) Deliver(_ context.Context, event bridge.Event[string]) error {
	fmt.Printf("processed %s: %s\n", event.Metadata.ID, event.Payload)
	return nil
}
