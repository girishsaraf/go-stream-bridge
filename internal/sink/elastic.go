package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
	"gostreambridge/internal/retry"
)

type document struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ElasticSink indexes messages in Elasticsearch. The client is created once
// and reused for the lifetime of the process.
type ElasticSink struct {
	client *elasticsearch.Client
	index  string
}

// NewElasticSink loads the Elasticsearch configuration and creates a client.
func NewElasticSink(configDir string) (*ElasticSink, error) {
	cfg, err := config.Load[config.Elasticsearch](config.File(configDir, "elastic.json"))
	if err != nil {
		return nil, err
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{cfg.URL},
		Username:  cfg.Username,
		Password:  cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create elasticsearch client: %w", err)
	}

	return &ElasticSink{client: client, index: cfg.Index}, nil
}

// Write indexes one message, retrying transient failures with backoff.
func (s *ElasticSink) Write(ctx context.Context, message []byte) error {
	payload, err := json.Marshal(document{
		Message:   string(message),
		Timestamp: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("marshal elasticsearch message: %w", err)
	}

	policy := retry.Default()
	policy.OnRetry = func(attempt int, err error) {
		log.Warn().
			Err(err).
			Str("index", s.index).
			Int("attempt", attempt).
			Msg("failed to index message")
	}

	return policy.Do(ctx, func(ctx context.Context) error {
		return s.indexDocument(ctx, payload)
	})
}

// Close is a no-op: the Elasticsearch client has no close method.
func (s *ElasticSink) Close() error {
	return nil
}

func (s *ElasticSink) indexDocument(ctx context.Context, payload []byte) error {
	req := esapi.IndexRequest{
		Index:   s.index,
		Body:    bytes.NewReader(payload),
		Refresh: "false",
	}

	res, err := req.Do(ctx, s.client)
	if err != nil {
		return fmt.Errorf("perform elasticsearch index request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if readErr != nil {
			return fmt.Errorf("index document: %s (read response body: %w)", res.Status(), readErr)
		}
		return fmt.Errorf("index document: %s: %s", res.Status(), strings.TrimSpace(string(body)))
	}

	return nil
}
