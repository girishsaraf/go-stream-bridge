package processors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"

	"gostreambridge/pkg/util"
)

// ConsumeKafkaMessages consumes messages from Kafka until the context is
// cancelled or a fatal error occurs. The handler is called synchronously for
// each message and the offset is committed only after the handler succeeds.
func ConsumeKafkaMessages(ctx context.Context, handler func([]byte) error) error {
	// Reading configuration
	kafkaConfig, err := util.ConvertConfigFileToMap("kafka_consumer.json")
	if err != nil {
		return fmt.Errorf("load kafka consumer config: %w", err)
	}

	brokers := kafkaConfig["brokers"]
	if brokers == "" {
		return errors.New("kafka consumer config: brokers must not be empty")
	}

	topic := kafkaConfig["topic"]
	if topic == "" {
		return errors.New("kafka consumer config: topic must not be empty")
	}

	// Kafka consumer configuration
	config := kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"group.id":           kafkaConfig["groupId"],
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false",
	}

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(&config)
	if err != nil {
		return fmt.Errorf("create kafka consumer: %w", err)
	}
	// This defer now runs when the consume loop exits, not when the function
	// hands a channel back to the caller.
	defer consumer.Close()

	// Subscribe to topic(s)
	if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
		return fmt.Errorf("subscribe to kafka topic %q: %w", topic, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := consumer.ReadMessage(500 * time.Millisecond)
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

		if err := handler(msg.Value); err != nil {
			// A write interrupted by shutdown is not a failure; leaving the
			// offset uncommitted means the message is replayed on restart.
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("process kafka message %s: %w", msg.TopicPartition, err)
		}

		if _, err := consumer.CommitMessage(msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit kafka message %s: %w", msg.TopicPartition, err)
		}
	}
}
