package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/rs/zerolog/log"
)

const kafkaFlushTimeout = 5 * time.Second

// KafkaWriter publishes dead-letter records to a Kafka topic.
type KafkaWriter struct {
	producer *kafka.Producer
	topic    string
}

// NewKafkaWriter creates a long-lived producer for the DLQ topic.
func NewKafkaWriter(brokers, topic string) (*KafkaWriter, error) {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"acks":               "all",
		"enable.idempotence": true,
		"message.timeout.ms": 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka DLQ producer: %w", err)
	}
	return &KafkaWriter{producer: producer, topic: topic}, nil
}

// Write publishes one JSON record and waits for its delivery report.
func (w *KafkaWriter) Write(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal DLQ record: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)
	if err := w.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &w.topic, Partition: kafka.PartitionAny},
		Value:          payload,
	}, deliveryChan); err != nil {
		return fmt.Errorf("produce DLQ record: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-deliveryChan:
		message, ok := event.(*kafka.Message)
		if !ok {
			return fmt.Errorf("unexpected DLQ delivery event: %T", event)
		}
		if message.TopicPartition.Error != nil {
			return fmt.Errorf("deliver DLQ record to %q: %w", w.topic, message.TopicPartition.Error)
		}
		return nil
	}
}

// Close flushes pending records and releases the producer.
func (w *KafkaWriter) Close() error {
	remaining := w.producer.Flush(int(kafkaFlushTimeout / time.Millisecond))
	w.producer.Close()
	if remaining > 0 {
		log.Warn().
			Int("remaining", remaining).
			Msg("kafka DLQ producer closed with outstanding messages or requests")
	}
	return nil
}
