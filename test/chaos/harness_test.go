//go:build chaos

// Package chaos contains opt-in fault-injection scenarios. It intentionally
// has no Docker or third-party dependency so `go test -tags chaos ./test/chaos`
// remains useful on a developer workstation.
package chaos

import (
	"context"
	"errors"
	"testing"
	"time"

	"gostreambridge/internal/testkit"
	"gostreambridge/pkg/bridge"
)

func TestContextCancellationInterruptsBlockedSink(t *testing.T) {
	sink := testkit.NewSink[string]()
	sink.SetGate(make(chan struct{}))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := sink.Deliver(ctx, bridge.Event[string]{Payload: "chaos"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Deliver error = %v, want deadline exceeded", err)
	}
}
