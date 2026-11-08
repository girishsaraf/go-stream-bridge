package dsprocessors

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

	"gostreambridge/pkg/util"
)

const (
	elasticMaxRetries = 5
	elasticRetryDelay = 2 * time.Second
)

type Message struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func NewElasticsearchClient(elasticConfig map[string]string) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{elasticConfig["url"]},
		Username:  elasticConfig["username"],
		Password:  elasticConfig["password"],
	}

	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return esClient, nil
}

// WriteToElastic indexes a message in Elasticsearch, retrying transient
// failures with a bounded number of attempts.
func WriteToElastic(ctx context.Context, initialMessage string) error {
	elasticConfig, err := util.ConvertConfigFileToMap("elastic.json")
	if err != nil {
		return fmt.Errorf("load elastic config: %w", err)
	}

	if elasticConfig["url"] == "" {
		return fmt.Errorf("elastic config: url must not be empty")
	}

	index := elasticConfig["index"]
	if index == "" {
		return fmt.Errorf("elastic config: index must not be empty")
	}

	esClient, err := NewElasticsearchClient(elasticConfig)
	if err != nil {
		return fmt.Errorf("create elasticsearch client: %w", err)
	}

	message := Message{
		Message:   initialMessage,
		Timestamp: time.Now(),
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal elastic message: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= elasticMaxRetries; attempt++ {
		indexErr := indexMessage(ctx, esClient, index, messageJSON)
		if indexErr == nil {
			return nil
		}
		lastErr = indexErr

		if attempt == elasticMaxRetries {
			break
		}

		log.Warn().
			Err(lastErr).
			Str("index", index).
			Int("attempt", attempt).
			Int("maxAttempts", elasticMaxRetries).
			Msg("failed to index message in elasticsearch")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(elasticRetryDelay):
		}
	}

	return fmt.Errorf("index message in elasticsearch after %d attempts: %w", elasticMaxRetries, lastErr)
}

// indexMessage indexes the message JSON to the specified Elasticsearch index.
func indexMessage(ctx context.Context, client *elasticsearch.Client, indexName string, messageJSON []byte) error {
	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: "", // Document ID is optional, Elasticsearch generates one if not provided
		Body:       bytes.NewReader(messageJSON),
		Refresh:    "false",
	}

	res, err := req.Do(ctx, client)
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
