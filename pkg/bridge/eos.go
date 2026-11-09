package bridge

import "context"

// TransactionalSource is a source that can participate in a compatible
// broker-level transaction.
type TransactionalSource[T any] interface {
	Source[T]
	Begin(context.Context) (SourceTransaction[T], error)
}

// SourceTransaction provides transactional source progress.
type SourceTransaction[T any] interface {
	Receive(context.Context) (Delivery[T], error)
	CommitOffsets(context.Context) error
	Abort(context.Context) error
}

// TransactionalSink is a sink that can participate in a compatible
// transaction.
type TransactionalSink[T any] interface {
	Sink[T]
	Begin(context.Context) (SinkTransaction[T], error)
}

// SinkTransaction buffers writes until Commit.
type SinkTransaction[T any] interface {
	Deliver(context.Context, Event[T]) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

// IdempotencyKey derives a stable deduplication key for effectively-once
// routes.
type IdempotencyKey func(Metadata) string
