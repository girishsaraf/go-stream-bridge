package stream

import (
	"context"
	"fmt"

	"gostreambridge/internal/database/dsprocessors"
	"gostreambridge/internal/queue/processors"
)

// DownstreamProcessor defines the interface for processing messages for different downstream systems
type DownstreamProcessor interface {
	ProcessMessage(ctx context.Context, message string) error
}

// MySQLProcessor implements the DownstreamProcessor interface for MySQL
type MySQLProcessor struct{}

// ProcessMessage processes the message for MySQL
func (m MySQLProcessor) ProcessMessage(ctx context.Context, message string) error {
	return dsprocessors.WriteToDatabase(ctx, "mysql", "mysql.json", message)
}

// SQLServerProcessor implements the DownstreamProcessor interface for SQL Server
type SQLServerProcessor struct{}

// ProcessMessage processes the message for SQL Server
func (s SQLServerProcessor) ProcessMessage(ctx context.Context, message string) error {
	return dsprocessors.WriteToDatabase(ctx, "sqlserver", "sqlserver.json", message)
}

// ElasticProcessor implements the DownstreamProcessor interface for Elasticsearch
type ElasticProcessor struct{}

// ProcessMessage processes the message for Elasticsearch
func (e ElasticProcessor) ProcessMessage(ctx context.Context, message string) error {
	return dsprocessors.WriteToElastic(ctx, message)
}

// KafkaProcessor implements the DownstreamProcessor interface for Kafka
type KafkaProcessor struct{}

// ProcessMessage processes the message for Kafka
func (k KafkaProcessor) ProcessMessage(ctx context.Context, message string) error {
	return processors.WriteToKafka(ctx, message)
}

var downstreamProcessors = map[string]DownstreamProcessor{
	"mysql":     MySQLProcessor{},
	"sqlserver": SQLServerProcessor{},
	"elastic":   ElasticProcessor{},
	"kafka":     KafkaProcessor{},
}

// ValidateDownstream returns an error when the requested downstream app is not supported.
func ValidateDownstream(downstreamAppName string) error {
	if _, ok := downstreamProcessors[downstreamAppName]; !ok {
		return fmt.Errorf("unsupported downstream system: %s", downstreamAppName)
	}
	return nil
}

// DetermineDownstreamFlow determines the downstream flow based on the downstreamAppName and calls the appropriate processor
func DetermineDownstreamFlow(ctx context.Context, downstreamAppName string, message string) error {
	processor, ok := downstreamProcessors[downstreamAppName]
	if !ok {
		return fmt.Errorf("unsupported downstream system: %s", downstreamAppName)
	}

	return processor.ProcessMessage(ctx, message)
}
