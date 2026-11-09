// Package config loads and validates stream bridge configuration.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Kafka holds the configuration shared by Kafka producers and consumers.
type Kafka struct {
	Brokers string `json:"brokers"`
	Topic   string `json:"topic"`
	GroupID string `json:"groupId"`
}

// Validate checks the fields required by a Kafka producer.
func (c Kafka) Validate() error {
	if c.Brokers == "" {
		return errors.New("brokers must not be empty")
	}
	if c.Topic == "" {
		return errors.New("topic must not be empty")
	}
	return nil
}

// ValidateConsumer checks the fields required by a Kafka consumer.
func (c Kafka) ValidateConsumer() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.GroupID == "" {
		return errors.New("groupId must not be empty")
	}
	return nil
}

// Database holds the configuration shared by the SQL sinks.
type Database struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

// Validate checks the fields required to build a database connection string.
func (c Database) Validate() error {
	switch {
	case c.Username == "":
		return errors.New("username must not be empty")
	case c.Host == "":
		return errors.New("host must not be empty")
	case c.Port == "":
		return errors.New("port must not be empty")
	case c.Database == "":
		return errors.New("database must not be empty")
	default:
		return nil
	}
}

// Elasticsearch holds the configuration for the Elasticsearch sink.
type Elasticsearch struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Index    string `json:"index"`
}

// Validate checks the fields required by the Elasticsearch sink.
func (c Elasticsearch) Validate() error {
	if c.URL == "" {
		return errors.New("url must not be empty")
	}
	if c.Index == "" {
		return errors.New("index must not be empty")
	}
	return nil
}

// RabbitMQ holds the configuration for the RabbitMQ source.
type RabbitMQ struct {
	URL      string `json:"url"`
	Queue    string `json:"queue"`
	Prefetch int    `json:"prefetch"`
	// Durable defaults to true when omitted. RabbitMQ 4 rejects transient
	// non-exclusive queues by default, so disable durability only when the
	// broker explicitly permits transient queues.
	Durable    *bool `json:"durable"`
	AutoDelete bool  `json:"autoDelete"`
}

// Validate checks the fields required by the RabbitMQ source.
func (c RabbitMQ) Validate() error {
	if c.URL == "" {
		return errors.New("url must not be empty")
	}
	if c.Queue == "" {
		return errors.New("queue must not be empty")
	}
	if c.Prefetch < 0 {
		return errors.New("prefetch must not be negative")
	}
	return nil
}

// Duration is a time.Duration that marshals to and from JSON strings such as
// "30s". Bare integers are rejected so units are always explicit.
type Duration time.Duration

// UnmarshalJSON parses a duration string.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("duration must be a string such as \"30s\": %w", err)
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalJSON renders a duration as a JSON string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Std returns the value as a time.Duration.
func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

// RetryPolicy configures pipeline-level retries for retryable sink errors.
type RetryPolicy struct {
	Attempts  int      `json:"attempts"`
	BaseDelay Duration `json:"baseDelay"`
	MaxDelay  Duration `json:"maxDelay"`
}

// DLQ configures where messages are written after terminal failures.
type DLQ struct {
	Type    string `json:"type"`
	Brokers string `json:"brokers"`
	Topic   string `json:"topic"`
	Path    string `json:"path"`
}

// Bridge holds optional runtime tuning loaded from bridge.json.
type Bridge struct {
	Workers         int         `json:"workers"`
	Buffer          int         `json:"buffer"`
	MessageTimeout  Duration    `json:"messageTimeout"`
	ShutdownTimeout Duration    `json:"shutdownTimeout"`
	FailurePolicy   string      `json:"failurePolicy"`
	Retry           RetryPolicy `json:"retry"`
	DLQ             DLQ         `json:"dlq"`
}

// Failure policies supported by the pipeline.
const (
	FailurePolicyStop = "stop"
	FailurePolicyDLQ  = "dlq"
)

// DLQ writer types.
const (
	DLQTypeNone  = ""
	DLQTypeKafka = "kafka"
	DLQTypeFile  = "file"
)

// DefaultBridge returns safe, production-oriented runtime defaults.
func DefaultBridge() Bridge {
	return Bridge{
		Workers:         1,
		Buffer:          256,
		MessageTimeout:  Duration(30 * time.Second),
		ShutdownTimeout: Duration(30 * time.Second),
		FailurePolicy:   FailurePolicyStop,
		Retry: RetryPolicy{
			Attempts:  5,
			BaseDelay: Duration(2 * time.Second),
			MaxDelay:  Duration(30 * time.Second),
		},
	}
}

// Validate checks the runtime configuration.
func (c Bridge) Validate() error {
	if c.Workers < 1 {
		return errors.New("workers must be at least 1")
	}
	if c.Buffer < 0 {
		return errors.New("buffer must not be negative")
	}
	if c.MessageTimeout.Std() <= 0 {
		return errors.New("messageTimeout must be greater than zero")
	}
	if c.ShutdownTimeout.Std() <= 0 {
		return errors.New("shutdownTimeout must be greater than zero")
	}

	switch c.FailurePolicy {
	case FailurePolicyStop:
	case FailurePolicyDLQ:
		if c.DLQ.Type != DLQTypeKafka && c.DLQ.Type != DLQTypeFile {
			return errors.New("failurePolicy dlq requires dlq.type to be kafka or file")
		}
	default:
		return fmt.Errorf("failurePolicy must be %q or %q", FailurePolicyStop, FailurePolicyDLQ)
	}

	if c.Retry.Attempts < 1 {
		return errors.New("retry.attempts must be at least 1")
	}
	if c.Retry.BaseDelay.Std() <= 0 {
		return errors.New("retry.baseDelay must be greater than zero")
	}
	if c.Retry.MaxDelay.Std() < c.Retry.BaseDelay.Std() {
		return errors.New("retry.maxDelay must be greater than or equal to retry.baseDelay")
	}

	switch c.DLQ.Type {
	case DLQTypeNone:
	case DLQTypeKafka:
		if c.DLQ.Brokers == "" {
			return errors.New("dlq.brokers must not be empty for the kafka DLQ")
		}
		if c.DLQ.Topic == "" {
			return errors.New("dlq.topic must not be empty for the kafka DLQ")
		}
	case DLQTypeFile:
		if c.DLQ.Path == "" {
			return errors.New("dlq.path must not be empty for the file DLQ")
		}
	default:
		return fmt.Errorf("dlq.type must be empty, %q, or %q", DLQTypeKafka, DLQTypeFile)
	}

	return nil
}

// Validator is implemented by every configuration type loaded from disk.
type Validator interface {
	Validate() error
}

// Load reads, expands environment references in, parses, and validates a JSON
// configuration file.
func Load[T Validator](path string) (T, error) {
	var cfg T

	data, err := readExpanded(path)
	if err != nil {
		return cfg, err
	}

	if err := decode(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config %q: %w", path, err)
	}

	return cfg, nil
}

// LoadOptional behaves like Load but returns defaults when the file does not
// exist. Fields present in the file override the supplied defaults.
func LoadOptional[T Validator](path string, defaults T) (T, error) {
	data, err := readExpanded(path)
	if errors.Is(err, fs.ErrNotExist) {
		if validateErr := defaults.Validate(); validateErr != nil {
			return defaults, fmt.Errorf("validate default config: %w", validateErr)
		}
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}

	cfg := defaults
	if err := decode(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config %q: %w", path, err)
	}
	return cfg, nil
}

func decode[T any](data []byte, target *T) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

// LoadBridge loads optional bridge.json runtime tuning from dir.
func LoadBridge(dir string) (Bridge, error) {
	return LoadOptional(File(dir, "bridge.json"), DefaultBridge())
}

// File returns the path to a named configuration file inside dir.
func File(dir, name string) string {
	return filepath.Join(dir, name)
}

func readExpanded(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	expanded, err := ExpandEnv(data)
	if err != nil {
		return nil, fmt.Errorf("expand config %q: %w", path, err)
	}
	return expanded, nil
}

// ExpandEnv replaces ${NAME} references with environment values before JSON
// parsing. Values are JSON-escaped, so quotes and backslashes in secrets are
// safe. $$ produces a literal $. A reference to an unset variable is an error
// so deployments fail fast instead of starting with an empty credential.
func ExpandEnv(data []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(data))

	for i := 0; i < len(data); {
		if data[i] != '$' {
			out.WriteByte(data[i])
			i++
			continue
		}

		if i+1 < len(data) && data[i+1] == '$' {
			out.WriteByte('$')
			i += 2
			continue
		}

		if i+1 >= len(data) || data[i+1] != '{' {
			out.WriteByte('$')
			i++
			continue
		}

		end := bytes.IndexByte(data[i+2:], '}')
		if end < 0 {
			return nil, fmt.Errorf("unterminated environment variable reference at byte %d", i)
		}

		name := string(data[i+2 : i+2+end])
		if !validEnvName(name) {
			return nil, fmt.Errorf("invalid environment variable name %q", name)
		}

		value, ok := os.LookupEnv(name)
		if !ok {
			return nil, fmt.Errorf("environment variable %q is not set", name)
		}

		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode environment variable %q: %w", name, err)
		}
		out.Write(encoded[1 : len(encoded)-1])

		i += 2 + end + 1
	}

	return out.Bytes(), nil
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
