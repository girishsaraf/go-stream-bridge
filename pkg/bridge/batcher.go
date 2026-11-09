package bridge

import (
	"context"
	"errors"
	"sync"
	"time"
)

// BatchItem retains the source delivery until the batch outcome is known.
type BatchItem[T any] struct {
	Delivery Delivery[T]
	Bytes    int
}

// Batcher groups bounded items and invokes flush serially. It never completes
// deliveries itself; the caller must apply per-item outcomes before acking.
type Batcher[T any] struct {
	policy BatchPolicy
	flush  func(context.Context, []BatchItem[T]) error

	input chan BatchItem[T]
	done  chan struct{}
	once  sync.Once
}

// NewBatcher creates a batcher. flush must be idempotent for items retried
// after ambiguous transport failures.
func NewBatcher[T any](policy BatchPolicy, flush func(context.Context, []BatchItem[T]) error) (*Batcher[T], error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if flush == nil {
		return nil, errors.New("batch flush function is nil")
	}
	return &Batcher[T]{
		policy: policy,
		flush:  flush,
		input:  make(chan BatchItem[T], policy.MaxEvents),
		done:   make(chan struct{}),
	}, nil
}

// Submit blocks when the batcher's bounded input is full.
func (b *Batcher[T]) Submit(ctx context.Context, item BatchItem[T]) error {
	if item.Delivery == nil {
		return errors.New("batch item delivery is nil")
	}
	if item.Bytes < 0 {
		return errors.New("batch item bytes must not be negative")
	}
	if item.Bytes > b.policy.MaxBytes {
		return errors.New("batch item exceeds maximum bytes")
	}
	select {
	case b.input <- item:
		return nil
	case <-b.done:
		return errors.New("batcher is closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run flushes batches until Close is called or ctx is cancelled.
func (b *Batcher[T]) Run(ctx context.Context) error {
	defer close(b.done)
	timer := time.NewTimer(b.policy.MaxWait)
	defer timer.Stop()
	var batch []BatchItem[T]
	bytes := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := b.flush(ctx, batch); err != nil {
			return err
		}
		batch = nil
		bytes = 0
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(b.policy.MaxWait)
		return nil
	}
	for {
		select {
		case item, ok := <-b.input:
			if !ok {
				return flush()
			}
			if len(batch) > 0 && (len(batch) == b.policy.MaxEvents || bytes+item.Bytes > b.policy.MaxBytes) {
				if err := flush(); err != nil {
					return err
				}
			}
			batch = append(batch, item)
			bytes += item.Bytes
			if len(batch) == b.policy.MaxEvents || bytes >= b.policy.MaxBytes {
				if err := flush(); err != nil {
					return err
				}
			}
		case <-timer.C:
			if err := flush(); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Close stops acceptance and flushes accepted items through Run.
func (b *Batcher[T]) Close() {
	b.once.Do(func() { close(b.input) })
}
