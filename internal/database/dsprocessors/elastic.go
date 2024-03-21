package dsprocessors

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"gostreambridge/pkg/util"
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

func WriteToElastic(initialMessage string) error {

	// Reading configuration
	elasticConfig, _ := util.ConvertConfigFileToMap("elastic.json")
	index, err := elasticConfig["index"]

	// Initialize Elasticsearch client
	esClient, clientErr := NewElasticsearchClient(elasticConfig)
	if clientErr != nil {
		log.Fatalf("Error creating Elasticsearch client: %s", err)
	}

	// Create a message to be indexed and add timestamp to the message
	message := Message{
		Message:   initialMessage,
		Timestamp: time.Now(),
	}

	// Marshal the message struct to JSON
	messageJSON, jsonErr := json.Marshal(message)
	if jsonErr != nil {
		log.Fatalf("Error marshaling message to JSON: %s", err)
	}

	maxRetries := 5
	// Retry indexing message with specified number of retries
	for i := 0; i < maxRetries; i++ {
		if err := indexMessage(esClient, index, string(messageJSON)); err != nil {
			log.Printf("Error indexing message (attempt %d/%d): %s\n", i+1, maxRetries, err)
			time.Sleep(2 * time.Second) // Wait before retrying
			continue
		}
		log.Println("Message indexed successfully")
		return nil
	}

	return errors.New("failed to index message after maximum retries")
}

// indexMessage indexes the message JSON to the specified Elasticsearch index
func indexMessage(client *elasticsearch.Client, indexName, messageJSON string) error {
	// Create index request
	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: "", // Document ID is optional, Elasticsearch generates one if not provided
		Body:       strings.NewReader(messageJSON),
		Refresh:    "true",
	}

	// Perform the request
	res, err := req.Do(context.Background(), client)
	if err != nil {
		log.Fatalf("Error performing request: %s", err)
	}
	defer res.Body.Close()

	// Check response status
	if res.IsError() {
		log.Fatalf("Error indexing document: %s", res.Status())
	}

	return nil
}
