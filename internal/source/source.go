// Package source provides long-lived upstream message consumers.
package source

import "context"

// Message is one upstream delivery. Ack must be called at most once after a
// terminal outcome: Ack(nil) acknowledges or commits the message, while
// Ack(err) leaves it unacknowledged so the broker can redeliver it.
type Message struct {
	Value       []byte
	Source      string
	Topic       string
	Partition   int32
	Offset      int64
	Queue       string
	DeliveryTag uint64
	Ack         func(err error) error
}

// Deliver hands a message to the processing pipeline. Returning an error stops
// consumption; the message is left unacknowledged.
type Deliver func(Message) error

// Source consumes messages and invokes deliver for each one. Consume blocks
// until the context is cancelled or a fatal error occurs. Close releases all
// resources owned by the source.
type Source interface {
	Consume(ctx context.Context, deliver Deliver) error
	Close() error
}

var (
	_ Source = (*KafkaSource)(nil)
	_ Source = (*RabbitMQSource)(nil)
)
