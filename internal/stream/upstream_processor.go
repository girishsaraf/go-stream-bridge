package stream

import (
	"gostreambridge/internal/queue/processors"
	"log"
)

// KafkaStreamBridge implements the StreamBridge interface for Kafka
type KafkaStreamBridge struct{}

// ConsumeMessages consumes messages from Kafka
func (k KafkaStreamBridge) ConsumeMessages(bridgeDetails map[string]string) {
	messages := processors.ConsumeKafkaMessages()
	for msg := range messages {
		ProcessMessage(string(msg.Value), bridgeDetails["downstreamApp"])
	}
}

// RabbitMQStreamBridge implements the StreamBridge interface for RabbitMQ
type RabbitMQStreamBridge struct{}

// ConsumeMessages consumes messages from RabbitMQ
func (r RabbitMQStreamBridge) ConsumeMessages(bridgeDetails map[string]string) {
	messages := processors.ConsumeAMQPMessages()
	for msg := range messages {
		ProcessMessage(string(msg), bridgeDetails["downstreamApp"])
	}
}

// ProcessMessage processes each message
func ProcessMessage(msg string, downstreamApp string) {
	err := DetermineDownstreamFlow(downstreamApp, msg)
	if err != nil {
		log.Printf("Error processing message: %v", err)
		return
	}
	log.Printf("Processed message: %s\n", string(msg))
}
