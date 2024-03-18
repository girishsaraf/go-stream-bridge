package database

import (
	"context"
	"encoding/json"
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

func WriteToElastic(initialMessage string) {

	// Reading configuration
	elasticConfig := util.ConvertConfigFileToMap("elastic.json")
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

	// Add the message JSON to Elasticsearch index
	if err := indexMessage(esClient, index, string(messageJSON)); err != nil {
		log.Fatalf("Error indexing message: %s", err)
	}

	log.Println("Message indexed successfully")
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
