package bridge

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBatcherFlushesAtConfiguredSize(t *testing.T) {
	var mu sync.Mutex
	var flushed [][]BatchItem[string]
	batcher, err := NewBatcher(BatchPolicy{
		MaxEvents: 2, MaxBytes: 16, MaxWait: time.Second, MaxInFlightBatches: 1,
	}, func(_ context.Context, items []BatchItem[string]) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, append([]BatchItem[string](nil), items...))
		return nil
	})
	if err != nil {
		t.Fatalf("NewBatcher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- batcher.Run(ctx) }()
	for range 2 {
		if err := batcher.Submit(context.Background(), BatchItem[string]{Delivery: &memoryDelivery{}}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	batcher.Close()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 || len(flushed[0]) != 2 {
		t.Fatalf("flushes = %#v", flushed)
	}
}
