package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"gostreambridge/internal/config"
)

const elasticStartupTimeout = 15 * time.Second

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

// NewElasticSink loads the Elasticsearch configuration, creates a client, and
// verifies connectivity so misconfiguration fails at startup.
func NewElasticSink(ctx context.Context, configDir string) (*ElasticSink, error) {
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

	s := &ElasticSink{client: client, index: cfg.Index}
	pingCtx, cancel := context.WithTimeout(ctx, elasticStartupTimeout)
	defer cancel()
	if err := s.ping(pingCtx); err != nil {
		return nil, err
	}
	return s, nil
}

// Write indexes one message. Retrying is handled centrally by the pipeline;
// this method performs exactly one attempt and classifies permanent failures.
func (s *ElasticSink) Write(ctx context.Context, message []byte) error {
	payload, err := json.Marshal(document{
		Message:   string(message),
		Timestamp: time.Now(),
	})
	if err != nil {
		return Permanent(fmt.Errorf("marshal elasticsearch message: %w", err))
	}

	return s.indexDocument(ctx, payload)
}

// Close is a no-op: the Elasticsearch client has no close method.
func (s *ElasticSink) Close() error {
	return nil
}

func (s *ElasticSink) ping(ctx context.Context) error {
	res, err := esapi.PingRequest{}.Do(ctx, s.client)
	if err != nil {
		return fmt.Errorf("ping elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return fmt.Errorf("ping elasticsearch: %s: %s", res.Status(), strings.TrimSpace(string(body)))
	}
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

		statusErr := &elasticStatusError{
			status:  res.StatusCode,
			message: fmt.Sprintf("index document: %s: %s", res.Status(), strings.TrimSpace(string(body))),
		}
		if isPermanentElasticStatus(res.StatusCode) {
			return Permanent(statusErr)
		}
		return statusErr
	}

	return nil
}

type elasticStatusError struct {
	status  int
	message string
}

func (e *elasticStatusError) Error() string { return e.message }

// isPermanentElasticStatus treats client errors as permanent except request
// timeout and too-many-requests, which can succeed after backoff.
func isPermanentElasticStatus(status int) bool {
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		return false
	}
	return status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}
