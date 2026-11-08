package stream

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"gostreambridge/internal/queue/processors"
)

// KafkaStreamBridge implements the StreamBridge interface for Kafka
type KafkaStreamBridge struct{}

// ConsumeMessages consumes messages from Kafka
func (k KafkaStreamBridge) ConsumeMessages(ctx context.Context, downstreamApp string) error {
	return processors.ConsumeKafkaMessages(ctx, func(value []byte) error {
		return ProcessMessage(ctx, string(value), downstreamApp)
	})
}

// RabbitMQStreamBridge implements the StreamBridge interface for RabbitMQ
type RabbitMQStreamBridge struct{}

// ConsumeMessages consumes messages from RabbitMQ
func (r RabbitMQStreamBridge) ConsumeMessages(ctx context.Context, downstreamApp string) error {
	return processors.ConsumeAMQPMessages(ctx, func(value []byte) error {
		return ProcessMessage(ctx, string(value), downstreamApp)
	})
}

// ProcessMessage processes each message
func ProcessMessage(ctx context.Context, msg string, downstreamApp string) error {
	if err := DetermineDownstreamFlow(ctx, downstreamApp, msg); err != nil {
		return fmt.Errorf("process message: %w", err)
	}
	log.Debug().Msg("processed message")
	return nil
}
