// Package dlq writes messages that exhausted retries to a dead-letter
// destination so the source can safely acknowledge them.
package dlq

import (
	"context"
	"fmt"
	"time"

	"gostreambridge/internal/config"
)

// Record describes a message that could not be delivered downstream.
type Record struct {
	Source      string    `json:"source"`
	Topic       string    `json:"topic,omitempty"`
	Partition   int32     `json:"partition"`
	Offset      int64     `json:"offset"`
	Queue       string    `json:"queue,omitempty"`
	DeliveryTag uint64    `json:"deliveryTag"`
	Attempts    int       `json:"attempts"`
	Error       string    `json:"error"`
	Timestamp   time.Time `json:"timestamp"`
	Payload     []byte    `json:"payload"`
}

// Writer persists one dead-letter record.
type Writer interface {
	Write(ctx context.Context, record Record) error
	Close() error
}

// New creates the configured DLQ writer. A disabled DLQ returns a nil writer.
func New(cfg config.DLQ) (Writer, error) {
	switch cfg.Type {
	case config.DLQTypeNone:
		return nil, nil
	case config.DLQTypeKafka:
		return NewKafkaWriter(cfg.Brokers, cfg.Topic)
	case config.DLQTypeFile:
		return NewFileWriter(cfg.Path)
	default:
		return nil, fmt.Errorf("unsupported DLQ type: %s", cfg.Type)
	}
}
