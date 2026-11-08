package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	URL   string `json:"url"`
	Queue string `json:"queue"`
}

// Validate checks the fields required by the RabbitMQ source.
func (c RabbitMQ) Validate() error {
	if c.URL == "" {
		return errors.New("url must not be empty")
	}
	if c.Queue == "" {
		return errors.New("queue must not be empty")
	}
	return nil
}

// Validator is implemented by every configuration type loaded from disk.
type Validator interface {
	Validate() error
}

// Load reads, parses, and validates a JSON configuration file.
func Load[T Validator](path string) (T, error) {
	var cfg T

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config %q: %w", path, err)
	}

	return cfg, nil
}

// File returns the path to a named configuration file inside dir.
func File(dir, name string) string {
	return filepath.Join(dir, name)
}
