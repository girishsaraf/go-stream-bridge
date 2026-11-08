package util

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConvertConfigFileToMapKafkaKeys guards the key names shared by the
// Kafka producer and consumer. A singular/plural mismatch here previously
// produced an empty bootstrap.servers value.
func TestConvertConfigFileToMapKafkaKeys(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	config, err := ConvertConfigFileToMap("kafka_consumer.json")
	if err != nil {
		t.Fatalf("ConvertConfigFileToMap: %v", err)
	}

	if config["brokers"] == "" {
		t.Error("expected a non-empty \"brokers\" key")
	}
	if config["topic"] == "" {
		t.Error("expected a non-empty \"topic\" key")
	}
	if _, ok := config["broker"]; ok {
		t.Error("unexpected singular \"broker\" key")
	}
}
