package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKafkaConsumerConfig(t *testing.T) {
	cfg, err := Load[Kafka](File("files", "kafka_consumer.json"))
	if err != nil {
		t.Fatalf("Load kafka_consumer.json: %v", err)
	}

	if cfg.Brokers == "" {
		t.Error("expected a non-empty brokers value")
	}
	if cfg.Topic == "" {
		t.Error("expected a non-empty topic value")
	}
	if cfg.GroupID == "" {
		t.Error("expected a non-empty groupId value")
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	if _, err := Load[Kafka](path); err == nil {
		t.Error("expected loading an empty Kafka config to fail")
	}
}
