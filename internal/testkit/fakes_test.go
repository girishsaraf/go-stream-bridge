package testkit

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"gostreambridge/pkg/bridge"
)

func TestSourceReceive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(context.Context) error
		want error
	}{
		{
			name: "scripted error precedes event",
			run: func(ctx context.Context) error {
				source := NewSource(bridge.Event[string]{Payload: "event"})
				want := errors.New("temporary receive error")
				source.FailNext(want)
				if _, err := source.Receive(ctx); !errors.Is(err, want) {
					return err
				}
				delivery, err := source.Receive(ctx)
				if err != nil {
					return err
				}
				if delivery.Event().Payload != "event" {
					return errors.New("unexpected event")
				}
				_, err = source.Receive(ctx)
				return err
			},
			want: io.EOF,
		},
		{
			name: "paused source respects cancellation",
			run: func(context.Context) error {
				source := NewSource(bridge.Event[string]{Payload: "event"})
				if err := source.Pause(context.Background()); err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				_, err := source.Receive(ctx)
				return err
			},
			want: context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(context.Background()); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSinkDeliver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Sink[string]) error
		want error
	}{
		{
			name: "returns scripted errors in order",
			run: func(sink *Sink[string]) error {
				want := errors.New("retryable")
				sink.FailNext(want)
				if err := sink.Deliver(context.Background(), bridge.Event[string]{Payload: "first"}); !errors.Is(err, want) {
					return err
				}
				return sink.Deliver(context.Background(), bridge.Event[string]{Payload: "second"})
			},
		},
		{
			name: "gate respects cancellation",
			run: func(sink *Sink[string]) error {
				sink.SetGate(make(chan struct{}))
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				return sink.Deliver(ctx, bridge.Event[string]{Payload: "blocked"})
			},
			want: context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := NewSink[string]()
			err := test.run(sink)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if sink.DeliveryCount() == 0 {
				t.Fatal("expected delivery attempt to be recorded")
			}
		})
	}
}

func TestDeliveryCompletion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Delivery[string]) error
		want error
	}{
		{
			name: "ack completes once",
			run: func(delivery *Delivery[string]) error {
				if err := delivery.Ack(context.Background()); err != nil {
					return err
				}
				return delivery.Nack(context.Background(), errors.New("late"))
			},
			want: ErrAlreadyCompleted,
		},
		{
			name: "cancelled acknowledgment does not complete",
			run: func(delivery *Delivery[string]) error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return delivery.Ack(ctx)
			},
			want: context.Canceled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			delivery := NewDelivery(bridge.Event[string]{Payload: "event"})
			if err := test.run(delivery); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func ExampleSink_SetGate() {
	sink := NewSink[string]()
	gate := make(chan struct{})
	sink.SetGate(gate)

	done := make(chan error, 1)
	go func() {
		done <- sink.Deliver(context.Background(), bridge.Event[string]{Payload: "event"})
	}()

	close(gate)
	<-done
	// Output:
}
