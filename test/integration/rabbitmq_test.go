//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gostreambridge/internal/config"
	"gostreambridge/internal/dlq"
)

func TestRabbitMQToKafka(t *testing.T) {
	brokers := kafkaBrokers(t)
	rabbit := rabbitURL(t)

	queue := uniqueName("rabbit-happy")
	outputTopic := uniqueName("rabbit-output")

	var values []string
	for i := 0; i < 5; i++ {
		values = append(values, fmt.Sprintf("rabbit-%02d", i))
	}
	publishRabbit(t, rabbit, queue, values...)

	configDir := writeConfigDir(t, map[string]any{
		"rabbitmq.json":       rabbitConfig(rabbit, queue),
		"kafka_producer.json": kafkaProducerConfig(brokers, outputTopic),
		"bridge.json":         bridgeConfig(1, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "rabbitmq", "kafka")
	got := consumeKafka(t, brokers, outputTopic, uniqueName("assert-group"), len(values), 45*time.Second)
	requireSameMessages(t, values, got)

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForRabbitQueueEmpty(t, rabbit, queue)
}

func TestRabbitMQDeadLetterAndAck(t *testing.T) {
	brokers := kafkaBrokers(t)
	rabbit := rabbitURL(t)
	esURL := elasticURL(t)

	queue := uniqueName("rabbit-dlq")
	dlqTopic := uniqueName("rabbit-dlq-topic")
	invalidIndex := "_invalid-" + uniqueName("index")

	values := []string{"rabbit-poison-one", "rabbit-poison-two"}
	publishRabbit(t, rabbit, queue, values...)

	configDir := writeConfigDir(t, map[string]any{
		"rabbitmq.json": rabbitConfig(rabbit, queue),
		"elastic.json":  elasticConfig(esURL, invalidIndex),
		"bridge.json": bridgeConfig(1, dlqPolicy(), map[string]any{
			"type":    config.DLQTypeKafka,
			"brokers": brokers,
			"topic":   dlqTopic,
		}),
	})

	bridge := startBridge(t, configDir, "rabbitmq", "elastic")
	records := consumeKafka(t, brokers, dlqTopic, uniqueName("dlq-assert"), len(values), 45*time.Second)

	var payloads []string
	for _, raw := range records {
		var record dlq.Record
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			t.Fatalf("parse DLQ record: %v", err)
		}
		payloads = append(payloads, string(record.Payload))
		if record.Source != "rabbitmq" || record.Queue != queue {
			t.Errorf("unexpected DLQ metadata: %+v", record)
		}
	}
	requireSameMessages(t, values, payloads)

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}

	// Dead-lettered deliveries are acknowledged, so nothing remains queued.
	waitForRabbitQueueEmpty(t, rabbit, queue)
}
