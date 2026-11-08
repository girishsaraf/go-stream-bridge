//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gostreambridge/internal/config"
	"gostreambridge/internal/dlq"
)

func TestKafkaToKafka(t *testing.T) {
	brokers := kafkaBrokers(t)
	inputTopic := uniqueName("kafka-input")
	outputTopic := uniqueName("kafka-output")
	group := uniqueName("kafka-group")

	var values []string
	for i := 0; i < 12; i++ {
		values = append(values, fmt.Sprintf("message-%02d", i))
	}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"kafka_producer.json": kafkaProducerConfig(brokers, outputTopic),
		"bridge.json":         bridgeConfig(2, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "kafka", "kafka")
	got := consumeKafka(t, brokers, outputTopic, uniqueName("assert-group"), len(values), 45*time.Second)
	requireSameMessages(t, values, got)

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, int64(len(values)))
}

func TestKafkaMultiWorkerContiguousCommits(t *testing.T) {
	brokers := kafkaBrokers(t)
	inputTopic := uniqueName("kafka-multi-input")
	outputTopic := uniqueName("kafka-multi-output")
	group := uniqueName("kafka-multi-group")

	var values []string
	for i := 0; i < 30; i++ {
		values = append(values, fmt.Sprintf("multi-%02d", i))
	}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"kafka_producer.json": kafkaProducerConfig(brokers, outputTopic),
		"bridge.json":         bridgeConfig(4, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "kafka", "kafka")
	got := consumeKafka(t, brokers, outputTopic, uniqueName("assert-group"), len(values), 60*time.Second)
	requireSameMessages(t, values, got)

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}

	// Out-of-order processing must still commit only contiguous positions.
	waitForCommittedOffsets(t, brokers, group, int64(len(values)))
}

func TestKafkaDeadLettersPermanentFailure(t *testing.T) {
	brokers := kafkaBrokers(t)
	esURL := elasticURL(t)

	inputTopic := uniqueName("kafka-dlq-input")
	dlqTopic := uniqueName("kafka-dlq-topic")
	group := uniqueName("kafka-dlq-group")
	invalidIndex := "_invalid-" + uniqueName("index")

	values := []string{"poison-one", "poison-two", "poison-three"}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"elastic.json":        elasticConfig(esURL, invalidIndex),
		"bridge.json": bridgeConfig(2, dlqPolicy(), map[string]any{
			"type":    config.DLQTypeKafka,
			"brokers": brokers,
			"topic":   dlqTopic,
		}),
	})

	bridge := startBridge(t, configDir, "kafka", "elastic")
	records := consumeKafka(t, brokers, dlqTopic, uniqueName("dlq-assert"), len(values), 45*time.Second)

	var deadLetters []dlq.Record
	var payloads []string
	for _, raw := range records {
		var record dlq.Record
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			t.Fatalf("parse DLQ record: %v\n%s", err, raw)
		}
		deadLetters = append(deadLetters, record)
		payloads = append(payloads, string(record.Payload))
	}
	requireSameMessages(t, values, payloads)

	for _, record := range deadLetters {
		if record.Source != "kafka" || record.Topic != inputTopic {
			t.Errorf("unexpected DLQ metadata: %+v", record)
		}
		if record.Attempts < 1 {
			t.Errorf("expected attempts >= 1, got %d", record.Attempts)
		}
		if record.Error == "" {
			t.Errorf("expected DLQ record to include the failure: %+v", record)
		}
	}

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, int64(len(values)))
}

func TestKafkaRestartReplaysUncommittedMessage(t *testing.T) {
	brokers := kafkaBrokers(t)
	esURL := elasticURL(t)

	inputTopic := uniqueName("kafka-replay-input")
	group := uniqueName("kafka-replay-group")
	validIndex := newElasticIndex("replay")
	invalidIndex := "_invalid-" + uniqueName("index")
	cleanupElasticIndex(t, esURL, validIndex)

	value := "replay-me"
	produceKafka(t, brokers, inputTopic, value)

	failingDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"elastic.json":        elasticConfig(esURL, invalidIndex),
		"bridge.json":         bridgeConfig(1, stopPolicy(), nil),
	})
	failing := startBridge(t, failingDir, "kafka", "elastic")
	if err := failing.wait(t, 45*time.Second); err == nil {
		t.Fatal("expected the first bridge run to fail")
	} else if !strings.Contains(strings.ToLower(err.Error()), "index") {
		t.Fatalf("expected an index failure, got %v", err)
	}

	recoveringDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"elastic.json":        elasticConfig(esURL, validIndex),
		"bridge.json":         bridgeConfig(1, stopPolicy(), nil),
	})
	recovering := startBridge(t, recoveringDir, "kafka", "elastic")
	waitForElasticCount(t, esURL, validIndex, 1)

	messages := elasticMessages(t, esURL, validIndex)
	if len(messages) != 1 || messages[0] != value {
		t.Fatalf("unexpected replayed documents: %v", messages)
	}

	if err := recovering.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, 1)
}

func TestKafkaGracefulShutdownCommitsProgress(t *testing.T) {
	brokers := kafkaBrokers(t)
	inputTopic := uniqueName("kafka-shutdown-input")
	outputTopic := uniqueName("kafka-shutdown-output")
	group := uniqueName("kafka-shutdown-group")

	var values []string
	for i := 0; i < 60; i++ {
		values = append(values, fmt.Sprintf("shutdown-%02d", i))
	}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"kafka_producer.json": kafkaProducerConfig(brokers, outputTopic),
		"bridge.json":         bridgeConfig(2, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "kafka", "kafka")
	waitFor(t, 20*time.Second, 200*time.Millisecond, "committing live progress", func() (bool, string) {
		committed := kafkaCommittedOffsets(t, brokers, group)
		return committed >= 5, fmt.Sprintf("committed offsets = %d", committed)
	})

	if err := bridge.stop(t); err != nil {
		t.Fatalf("graceful shutdown failed: %v", err)
	}

	committed := kafkaCommittedOffsets(t, brokers, group)
	if committed < 5 || committed > int64(len(values)) {
		t.Fatalf("unexpected committed offsets after shutdown: %d", committed)
	}
}
