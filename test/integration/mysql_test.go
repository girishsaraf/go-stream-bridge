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

func TestKafkaToMySQL(t *testing.T) {
	brokers := kafkaBrokers(t)
	settings := mysqlEnv(t)
	db := openMySQL(t, settings)
	applyMySQLSchema(t, db)
	truncateMySQL(t, db)

	inputTopic := uniqueName("mysql-input")
	group := uniqueName("mysql-group")

	var values []string
	for i := 0; i < 8; i++ {
		values = append(values, fmt.Sprintf("mysql-%02d", i))
	}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"mysql.json":          settings.config(),
		"bridge.json":         bridgeConfig(2, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "kafka", "mysql")
	waitForMySQLCount(t, db, len(values))
	requireSameMessages(t, values, mysqlMessages(t, db))

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, int64(len(values)))
}

func TestKafkaMySQLDeadLetter(t *testing.T) {
	brokers := kafkaBrokers(t)
	settings := mysqlEnv(t)
	db := openMySQL(t, settings)
	applyMySQLSchema(t, db)
	truncateMySQL(t, db)
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS messages")
		applyMySQLSchema(t, db)
	})

	inputTopic := uniqueName("mysql-dlq-input")
	dlqTopic := uniqueName("mysql-dlq-topic")
	group := uniqueName("mysql-dlq-group")

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"mysql.json":          settings.config(),
		"bridge.json": bridgeConfig(1, dlqPolicy(), map[string]any{
			"type":    config.DLQTypeKafka,
			"brokers": brokers,
			"topic":   dlqTopic,
		}),
	})

	bridge := startBridge(t, configDir, "kafka", "mysql")

	// Wait until the bridge is fully wired and writing successfully, then
	// remove the destination table to force a permanent SQL failure.
	produceKafka(t, brokers, inputTopic, "warmup")
	waitForMySQLCount(t, db, 1)
	if _, err := db.Exec("DROP TABLE messages"); err != nil {
		t.Fatalf("drop messages table: %v", err)
	}

	poison := []string{"poison-one", "poison-two"}
	produceKafka(t, brokers, inputTopic, poison...)

	records := consumeKafka(t, brokers, dlqTopic, uniqueName("dlq-assert"), len(poison), 45*time.Second)
	var payloads []string
	for _, raw := range records {
		var record dlq.Record
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			t.Fatalf("parse DLQ record: %v", err)
		}
		payloads = append(payloads, string(record.Payload))
		if record.Source != "kafka" || record.Topic != inputTopic {
			t.Errorf("unexpected DLQ metadata: %+v", record)
		}
		if !strings.Contains(record.Error, "1146") && !strings.Contains(strings.ToLower(record.Error), "doesn't exist") {
			t.Errorf("expected a missing-table error, got %q", record.Error)
		}
	}
	requireSameMessages(t, poison, payloads)

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, int64(1+len(poison)))
}
