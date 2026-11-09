package adapter

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"gostreambridge/internal/source"
	"gostreambridge/pkg/bridge"
)

type fakeSource struct {
	message source.Message
	send    bool
	closed  bool
}

func (s *fakeSource) Consume(ctx context.Context, deliver source.Deliver) error {
	if s.send {
		if err := deliver(s.message); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

func (s *fakeSource) Close() error {
	s.closed = true
	return nil
}

func TestSourceTranslatesAcknowledgements(t *testing.T) {
	var acknowledged error
	legacy := &fakeSource{send: true, message: source.Message{
		Value:  []byte("event"),
		Source: "legacy",
		Ack: func(err error) error {
			acknowledged = err
			return nil
		},
	}}
	adapted, err := NewSource(legacy)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := adapted.Open(ctx); err != nil {
		t.Fatal(err)
	}
	delivery, err := adapted.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(delivery.Event().Payload) != "event" {
		t.Fatalf("unexpected payload: %q", delivery.Event().Payload)
	}
	rejected := errors.New("failed")
	if err := delivery.Nack(context.Background(), rejected); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(acknowledged, rejected) {
		t.Fatalf("legacy acknowledgement received %v, want %v", acknowledged, rejected)
	}
	if err := delivery.Ack(context.Background()); !errors.Is(err, ErrDeliveryCompleted) {
		t.Fatalf("second acknowledgement error = %v, want %v", err, ErrDeliveryCompleted)
	}
	if err := adapted.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !legacy.closed {
		t.Fatal("legacy source was not closed")
	}
}

func TestSourceHonorsReceiveContextAndPause(t *testing.T) {
	adapted, err := NewSource(&fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapted.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapted.Receive(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Receive error = %v, want context cancellation", err)
	}
	if err := adapted.Pause(context.Background()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := adapted.Resume(context.Background()); !errors.Is(err, ErrResumeUnsupported) {
		t.Fatalf("Resume error = %v, want %v", err, ErrResumeUnsupported)
	}
	if err := adapted.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := adapted.Receive(context.Background()); !errors.Is(err, io.EOF) {
			t.Fatalf("Receive after close error = %v, want EOF", err)
		}
	}
}

func TestPauseKeepsInFlightDeliveryAcknowledgable(t *testing.T) {
	var acknowledged bool
	adapted, err := NewSource(&fakeSource{send: true, message: source.Message{
		Ack: func(error) error {
			acknowledged = true
			return nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapted.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	delivery, err := adapted.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := adapted.Pause(context.Background()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := delivery.Ack(context.Background()); err != nil {
		t.Fatalf("Ack after Pause: %v", err)
	}
	if !acknowledged {
		t.Fatal("in-flight delivery was not acknowledged")
	}
	if _, err := adapted.Receive(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("Receive after Pause error = %v, want EOF", err)
	}
}

type fakeBridgeSink struct{}

func (fakeBridgeSink) Open(context.Context) error                          { return nil }
func (fakeBridgeSink) Deliver(context.Context, bridge.Event[[]byte]) error { return nil }
func (fakeBridgeSink) Close(context.Context) error                         { return nil }

func TestPipelineShutdownCompletesWithLegacySource(t *testing.T) {
	started := make(chan struct{})
	legacy := &blockingSource{started: started}
	adapted, err := NewSource(legacy)
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := bridge.NewPipeline[[]byte](adapted, fakeBridgeSink{}, bridge.PipelineConfig{
		Workers:         1,
		Buffer:          0,
		MessageTimeout:  time.Second,
		ShutdownTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- pipeline.Run(context.Background()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("legacy source did not begin consuming")
	}
	if err := pipeline.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pipeline did not stop after Shutdown")
	}
}

type blockingSource struct {
	started chan struct{}
}

func (s *blockingSource) Consume(ctx context.Context, _ source.Deliver) error {
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}

func (*blockingSource) Close() error { return nil }

var _ bridge.Source[[]byte] = (*Source)(nil)
