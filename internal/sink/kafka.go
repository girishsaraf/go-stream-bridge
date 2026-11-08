package sink

import (
	"context"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
)

const kafkaFlushTimeout = 5 * time.Second

// KafkaSink writes messages to a Kafka topic. The producer is created once and
// reused for the lifetime of the process.
type KafkaSink struct {
	producer *kafka.Producer
	topic    string
}

// NewKafkaSink loads the producer configuration and creates a Kafka producer.
func NewKafkaSink(configDir string) (*KafkaSink, error) {
	cfg, err := config.Load[config.Kafka](config.File(configDir, "kafka_producer.json"))
	if err != nil {
		return nil, err
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  cfg.Brokers,
		"acks":               "all",
		"enable.idempotence": true,
		"message.timeout.ms": 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}

	return &KafkaSink{producer: producer, topic: cfg.Topic}, nil
}

// Write produces one message and waits for its delivery report. librdkafka
// retries transient delivery failures internally until message.timeout.ms.
func (s *KafkaSink) Write(ctx context.Context, message []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// A per-call channel keeps Write safe for concurrent use.
	deliveryChan := make(chan kafka.Event, 1)

	if err := s.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &s.topic, Partition: kafka.PartitionAny},
		Value:          message,
	}, deliveryChan); err != nil {
		return fmt.Errorf("produce kafka message: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-deliveryChan:
		delivered, ok := event.(*kafka.Message)
		if !ok {
			return fmt.Errorf("unexpected kafka delivery event: %T", event)
		}
		if delivered.TopicPartition.Error != nil {
			return fmt.Errorf("deliver kafka message to topic %q: %w", s.topic, delivered.TopicPartition.Error)
		}
		return nil
	}
}

// Close flushes pending messages and releases the producer. Flush also counts
// outstanding protocol requests, so a non-zero remainder is only a warning:
// it can occur when the broker is unreachable even if no messages are queued.
func (s *KafkaSink) Close() error {
	remaining := s.producer.Flush(int(kafkaFlushTimeout / time.Millisecond))
	s.producer.Close()

	if remaining > 0 {
		log.Warn().
			Int("remaining", remaining).
			Msg("kafka producer closed with outstanding messages or requests")
	}
	return nil
}
