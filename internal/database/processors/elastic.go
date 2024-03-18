package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	elasticConfig = util.ConvertConfigFileToMap("elastic.json")
	index, err := elasticConfig["index"]
	if err != nil {
		log.Fatalf("Error while reading config: %v", err)
	}

	// Initialize Elasticsearch client
	esClient, err := NewElasticsearchClient(elasticConfig)
	if err != nil {
		log.Fatalf("Error creating Elasticsearch client: %s", err)
	}
	
	// Initialize Elasticsearch client
	esClient, err := elasticsearch.NewDefaultClient()
	if err != nil {
		log.Fatalf("Error creating Elasticsearch client: %s", err)
	}

	// Create a message to be indexed and add timestamp to the message
	message := Message{
		Message:   initialMessage,
		Timestamp: time.Now(),
	}

	// Marshal the message struct to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Fatalf("Error marshaling message to JSON: %s", err)
	}

	// Add the message JSON to Elasticsearch index
	if err := indexMessage(esClient, indexName, string(messageJSON)); err != nil {
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
		Body:       esapi.BodyString(messageJSON),
		Refresh:    "true",
	}

	// Perform the request
	res, err := req.Do(context.Background(), client)
	if err != nil {
		return log.Errorf("Error performing request: %s", err)
	}
	defer res.Body.Close()

	// Check response status
	if res.IsError() {
		return log.Errorf("Error indexing document: %s", res.Status())
	}

	return nil
}