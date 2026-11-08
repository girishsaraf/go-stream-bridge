//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func kafkaBrokers(t *testing.T) string {
	t.Helper()
	return requireEnv(t, "KAFKA_BROKERS")
}

func produceKafka(t *testing.T, brokers, topic string, values ...string) {
	t.Helper()

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"message.timeout.ms": 30000,
	})
	if err != nil {
		t.Fatalf("create producer: %v", err)
	}
	defer producer.Close()

	deliveries := make(chan kafka.Event, len(values))
	for _, value := range values {
		message := &kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte(value),
		}
		if err := producer.Produce(message, deliveries); err != nil {
			t.Fatalf("produce to %s: %v", topic, err)
		}
	}

	for range values {
		select {
		case event := <-deliveries:
			message, ok := event.(*kafka.Message)
			if !ok {
				t.Fatalf("unexpected delivery event: %T", event)
			}
			if message.TopicPartition.Error != nil {
				t.Fatalf("deliver to %s: %v", topic, message.TopicPartition.Error)
			}
		case <-time.After(45 * time.Second):
			t.Fatalf("timed out producing %d messages to %s", len(values), topic)
		}
	}
	producer.Flush(5000)
}

func consumeKafka(t *testing.T, brokers, topic, group string, count int, timeout time.Duration) []string {
	t.Helper()

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"group.id":           group,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false",
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer consumer.Close()

	if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
		t.Fatalf("subscribe to %s: %v", topic, err)
	}

	deadline := time.Now().Add(timeout)
	var messages []string
	var lastErr error
	for len(messages) < count && time.Now().Before(deadline) {
		message, err := consumer.ReadMessage(250 * time.Millisecond)
		if err != nil {
			var kafkaErr kafka.Error
			if errors.As(err, &kafkaErr) {
				if kafkaErr.Code() == kafka.ErrTimedOut {
					continue
				}
				// Topics are auto-created by the sink, so the assertion
				// consumer may subscribe before the topic exists. Other
				// non-fatal errors (for example coordinator loading) are also
				// worth waiting out until the deadline.
				if !kafkaErr.IsFatal() {
					lastErr = err
					time.Sleep(250 * time.Millisecond)
					continue
				}
			}
			t.Fatalf("consume from %s: %v", topic, err)
		}
		messages = append(messages, string(message.Value))
	}

	if len(messages) < count {
		t.Fatalf("timed out consuming from %s: got %d of %d messages (last error: %v)", topic, len(messages), count, lastErr)
	}
	return messages
}

func kafkaCommittedOffsets(t *testing.T, brokers, group string) int64 {
	t.Helper()

	admin, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		t.Fatalf("create admin client: %v", err)
	}
	defer admin.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := admin.ListConsumerGroupOffsets(ctx, []kafka.ConsumerGroupTopicPartitions{{Group: group}})
	if err != nil {
		var kafkaErr kafka.Error
		if errors.As(err, &kafkaErr) && kafkaErr.Code() == kafka.ErrGroupIDNotFound {
			return 0
		}
		t.Fatalf("list offsets for group %s: %v", group, err)
	}
	if len(result.ConsumerGroupsTopicPartitions) == 0 {
		return 0
	}

	var total int64
	for _, partition := range result.ConsumerGroupsTopicPartitions[0].Partitions {
		if partition.Offset >= 0 {
			total += int64(partition.Offset)
		}
	}
	return total
}

func waitForCommittedOffsets(t *testing.T, brokers, group string, want int64) {
	t.Helper()

	waitFor(t, 20*time.Second, 300*time.Millisecond, fmt.Sprintf("group %s reaching %d committed offsets", group, want), func() (bool, string) {
		got := kafkaCommittedOffsets(t, brokers, group)
		return got == want, fmt.Sprintf("committed offsets = %d", got)
	})
}
