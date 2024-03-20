package stream

import (
	"errors"

	"gostreambridge/internal/database/dsprocessors"
	"gostreambridge/internal/queue/processors"
)

// DownstreamProcessor defines the interface for processing messages for different downstream systems
type DownstreamProcessor interface {
	ProcessMessage(message string) error
}

// MySQLProcessor implements the DownstreamProcessor interface for MySQL
type MySQLProcessor struct{}

// ProcessMessage processes the message for MySQL
func (m MySQLProcessor) ProcessMessage(message string) error {
	return dsprocessors.WriteToMySQL(message)
}

// SQLServerProcessor implements the DownstreamProcessor interface for SQL Server
type SQLServerProcessor struct{}

// ProcessMessage processes the message for SQL Server
func (s SQLServerProcessor) ProcessMessage(message string) error {
	return dsprocessors.WriteToSQLServer(message)
}

// ElasticProcessor implements the DownstreamProcessor interface for Elasticsearch
type ElasticProcessor struct{}

// ProcessMessage processes the message for Elasticsearch
func (e ElasticProcessor) ProcessMessage(message string) error {
	return dsprocessors.WriteToElastic(message)
}

// KafkaProcessor implements the DownstreamProcessor interface for Kafka
type KafkaProcessor struct{}

// ProcessMessage processes the message for Kafka
func (k KafkaProcessor) ProcessMessage(message string) error {
	return processors.WriteToKafka(message)
}

// DetermineDownstreamFlow determines the downstream flow based on the downstreamAppName and calls the appropriate processor
func DetermineDownstreamFlow(downstreamAppName string, message string) error {
	var processor DownstreamProcessor

	switch downstreamAppName {
	case "mysql":
		processor = MySQLProcessor{}
	case "sqlserver":
		processor = SQLServerProcessor{}
	case "elastic":
		processor = ElasticProcessor{}
	case "kafka":
		processor = KafkaProcessor{}
	default:
		return errors.New("unsupported downstream system")
	}

	return processor.ProcessMessage(message)
}
