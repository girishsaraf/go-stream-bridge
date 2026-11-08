package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
)

const kafkaPollTimeout = 500 * time.Millisecond

// KafkaSource consumes messages from a Kafka topic. The consumer is created
// once and reused for the lifetime of the process.
type KafkaSource struct {
	consumer *kafka.Consumer
}

// NewKafkaSource loads the consumer configuration and creates a subscribed
// Kafka consumer.
func NewKafkaSource(configDir string) (*KafkaSource, error) {
	cfg, err := config.Load[config.Kafka](config.File(configDir, "kafka_consumer.json"))
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateConsumer(); err != nil {
		return nil, fmt.Errorf("kafka consumer config: %w", err)
	}

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  cfg.Brokers,
		"group.id":           cfg.GroupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false",
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}

	if err := consumer.SubscribeTopics([]string{cfg.Topic}, nil); err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("subscribe to kafka topic %q: %w", cfg.Topic, err)
	}

	return &KafkaSource{consumer: consumer}, nil
}

// Consume reads messages until the context is cancelled. Offsets are committed
// only after the handler returns nil.
func (s *KafkaSource) Consume(ctx context.Context, handle Handler) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := s.consumer.ReadMessage(kafkaPollTimeout)
		if err != nil {
			var kafkaErr kafka.Error
			if errors.As(err, &kafkaErr) {
				if kafkaErr.Code() == kafka.ErrTimedOut {
					continue
				}
				if kafkaErr.IsFatal() {
					return fmt.Errorf("fatal kafka consumer error: %w", err)
				}
			}
			if ctx.Err() != nil {
				return nil
			}
			log.Error().Err(err).Msg("error reading message from kafka")
			continue
		}

		if err := handle(ctx, msg.Value); err != nil {
			// A write interrupted by shutdown is not a failure; leaving the
			// offset uncommitted means the message is replayed on restart.
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("process kafka message %s: %w", msg.TopicPartition, err)
		}

		if _, err := s.consumer.CommitMessage(msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit kafka message %s: %w", msg.TopicPartition, err)
		}
	}
}

// Close releases the Kafka consumer.
func (s *KafkaSource) Close() error {
	return s.consumer.Close()
}
