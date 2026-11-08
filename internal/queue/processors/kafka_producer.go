package processors

import (
	"context"
	"errors"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"

	"gostreambridge/pkg/util"
)

// WriteToKafka writes a message to the configured Kafka topic. Delivery
// failures are retried internally by librdkafka until message.timeout.ms.
func WriteToKafka(ctx context.Context, message string) error {
	kafkaConfig, err := util.ConvertConfigFileToMap("kafka_producer.json")
	if err != nil {
		return fmt.Errorf("load kafka producer config: %w", err)
	}

	brokers := kafkaConfig["brokers"]
	topic := kafkaConfig["topic"]
	if brokers == "" {
		return errors.New("kafka producer config: brokers must not be empty")
	}
	if topic == "" {
		return errors.New("kafka producer config: topic must not be empty")
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"acks":               "all",
		"enable.idempotence": true,
		"message.timeout.ms": 10000,
	})
	if err != nil {
		return fmt.Errorf("create kafka producer: %w", err)
	}
	defer producer.Close()

	// Buffered so the delivery report goroutine can never block if we return
	// early because the context was cancelled.
	deliveryChan := make(chan kafka.Event, 1)

	if err := producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte(message),
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
			return fmt.Errorf("deliver kafka message to topic %q: %w", topic, delivered.TopicPartition.Error)
		}
		return nil
	}
}
