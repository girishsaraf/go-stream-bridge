package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestExpandEnv(t *testing.T) {
	t.Setenv("STREAM_BRIDGE_TEST_SECRET", `p"a\ss`)

	data := []byte(`{"password":"${STREAM_BRIDGE_TEST_SECRET}","literal":"$$HOME","prefix":"x-${STREAM_BRIDGE_TEST_SECRET}"}`)
	expanded, err := ExpandEnv(data)
	if err != nil {
		t.Fatalf("ExpandEnv: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(expanded, &got); err != nil {
		t.Fatalf("expanded config is not valid JSON: %v\n%s", err, expanded)
	}

	if got["password"] != `p"a\ss` {
		t.Errorf("unexpected expanded password: %q", got["password"])
	}
	if got["prefix"] != `x-p"a\ss` {
		t.Errorf("unexpected expanded prefix: %q", got["prefix"])
	}
	if got["literal"] != "$HOME" {
		t.Errorf("expected $$ to produce a literal $, got %q", got["literal"])
	}
}

func TestExpandEnvReportsMissingVariable(t *testing.T) {
	const name = "STREAM_BRIDGE_TEST_DEFINITELY_UNSET"
	os.Unsetenv(name)

	_, err := ExpandEnv([]byte(`{"password":"${` + name + `}"}`))
	if err == nil {
		t.Fatal("expected missing environment variable to fail")
	}
	if !strings.Contains(err.Error(), name) {
		t.Errorf("error should name the variable: %v", err)
	}
}

func TestExpandEnvRejectsMalformedReferences(t *testing.T) {
	if _, err := ExpandEnv([]byte(`{"value":"${UNCLOSED"}`)); err == nil {
		t.Error("expected unterminated reference to fail")
	}
	if _, err := ExpandEnv([]byte(`{"value":"${BAD-NAME}"}`)); err == nil {
		t.Error("expected invalid variable name to fail")
	}
}

func TestLoadExpandsEnvironment(t *testing.T) {
	t.Setenv("STREAM_BRIDGE_TEST_BROKERS", "broker:9092")
	t.Setenv("STREAM_BRIDGE_TEST_TOPIC", "events")

	path := filepath.Join(t.TempDir(), "kafka.json")
	content := `{"brokers":"${STREAM_BRIDGE_TEST_BROKERS}","topic":"${STREAM_BRIDGE_TEST_TOPIC}","groupId":"group"}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load[Kafka](path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Brokers != "broker:9092" || cfg.Topic != "events" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRabbitMQSample(t *testing.T) {
	cfg, err := Load[RabbitMQ](File("files", "rabbitmq.json"))
	if err != nil {
		t.Fatalf("Load rabbitmq.json: %v", err)
	}
	if cfg.URL == "" || cfg.Queue == "" {
		t.Fatalf("unexpected sample config: %+v", cfg)
	}
	if cfg.Durable == nil || !*cfg.Durable {
		t.Fatal("sample RabbitMQ queue should be durable")
	}
}

func TestLoadBridgeDefaultsWhenMissing(t *testing.T) {
	cfg, err := LoadBridge(t.TempDir())
	if err != nil {
		t.Fatalf("LoadBridge: %v", err)
	}

	want := DefaultBridge()
	if cfg != want {
		t.Fatalf("expected defaults %+v, got %+v", want, cfg)
	}
}

func TestLoadBridgePreservesUnspecifiedDefaults(t *testing.T) {
	dir := t.TempDir()
	content := `{"workers":4,"retry":{"attempts":2}}`
	if err := os.WriteFile(filepath.Join(dir, "bridge.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write bridge.json: %v", err)
	}

	cfg, err := LoadBridge(dir)
	if err != nil {
		t.Fatalf("LoadBridge: %v", err)
	}

	if cfg.Workers != 4 {
		t.Errorf("expected workers 4, got %d", cfg.Workers)
	}
	if cfg.Retry.Attempts != 2 {
		t.Errorf("expected retry attempts 2, got %d", cfg.Retry.Attempts)
	}
	if cfg.Buffer != DefaultBridge().Buffer {
		t.Errorf("expected default buffer, got %d", cfg.Buffer)
	}
	if cfg.Retry.BaseDelay != DefaultBridge().Retry.BaseDelay {
		t.Errorf("expected default retry base delay, got %s", cfg.Retry.BaseDelay.Std())
	}
}

func TestLoadBridgeSample(t *testing.T) {
	cfg, err := LoadBridge("files")
	if err != nil {
		t.Fatalf("LoadBridge files: %v", err)
	}
	if cfg.Workers != 1 {
		t.Errorf("expected sample workers 1, got %d", cfg.Workers)
	}
	if cfg.FailurePolicy != FailurePolicyStop {
		t.Errorf("expected sample failurePolicy stop, got %q", cfg.FailurePolicy)
	}
}

func TestDurationUnmarshal(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"1m30s"`), &d); err != nil {
		t.Fatalf("unmarshal duration: %v", err)
	}
	if d.Std() != 90*time.Second {
		t.Fatalf("expected 90s, got %s", d.Std())
	}

	if err := json.Unmarshal([]byte(`30`), &d); err == nil {
		t.Fatal("expected a bare number to be rejected")
	}
	if err := json.Unmarshal([]byte(`"not-a-duration"`), &d); err == nil {
		t.Fatal("expected an invalid duration to be rejected")
	}

	encoded, err := json.Marshal(Duration(time.Minute))
	if err != nil {
		t.Fatalf("marshal duration: %v", err)
	}
	if string(encoded) != `"1m0s"` {
		t.Errorf("unexpected marshaled duration: %s", encoded)
	}
}

func TestBridgeValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Bridge)
	}{
		{name: "zero workers", mutate: func(c *Bridge) { c.Workers = 0 }},
		{name: "negative buffer", mutate: func(c *Bridge) { c.Buffer = -1 }},
		{name: "zero message timeout", mutate: func(c *Bridge) { c.MessageTimeout = 0 }},
		{name: "zero shutdown timeout", mutate: func(c *Bridge) { c.ShutdownTimeout = 0 }},
		{name: "unknown failure policy", mutate: func(c *Bridge) { c.FailurePolicy = "retry" }},
		{name: "dlq without writer", mutate: func(c *Bridge) {
			c.FailurePolicy = FailurePolicyDLQ
			c.DLQ = DLQ{}
		}},
		{name: "kafka dlq without brokers", mutate: func(c *Bridge) {
			c.DLQ = DLQ{Type: DLQTypeKafka, Topic: "dlq"}
		}},
		{name: "kafka dlq without topic", mutate: func(c *Bridge) {
			c.DLQ = DLQ{Type: DLQTypeKafka, Brokers: "localhost:9092"}
		}},
		{name: "file dlq without path", mutate: func(c *Bridge) {
			c.DLQ = DLQ{Type: DLQTypeFile}
		}},
		{name: "unknown dlq type", mutate: func(c *Bridge) { c.DLQ = DLQ{Type: "s3"} }},
		{name: "zero retry attempts", mutate: func(c *Bridge) { c.Retry.Attempts = 0 }},
		{name: "zero base delay", mutate: func(c *Bridge) { c.Retry.BaseDelay = 0 }},
		{name: "max delay below base", mutate: func(c *Bridge) { c.Retry.MaxDelay = c.Retry.BaseDelay / 2 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultBridge()
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
