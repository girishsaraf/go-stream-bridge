// Package source provides long-lived upstream message consumers.
package source

import "context"

// Handler processes a single message. Returning an error prevents the source
// from acknowledging or committing that message.
type Handler func(ctx context.Context, message []byte) error

// Source consumes messages and invokes Handler for each one. Consume blocks
// until the context is cancelled or a fatal error occurs. Close releases all
// resources owned by the source.
type Source interface {
	Consume(ctx context.Context, handle Handler) error
	Close() error
}

var (
	_ Source = (*KafkaSource)(nil)
	_ Source = (*RabbitMQSource)(nil)
)
