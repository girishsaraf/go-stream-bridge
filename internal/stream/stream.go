package stream

import (
	"syscall"
	"os"
	"os/signal"
	"log"
	"time"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"gostreambridge/internal/queue/processors"
)

func StartStreamBridge(upstreamQueueType string, downstreamApplication string) {
	ConsumeMessages(upstreamQueueType)
}

func ConsumeMessages(upstreamQueueType string) {

	switch upstreamQueueType {
		case "kafka":
			// Create a Kafka message consumer
			messages := processors.ConsumeKafkaMessages("", "", "", "")
			// Process each message
			for msg := range messages {
				ProcessKafkaMessage(msg)
			}
		case "rabbitmq":
			// Create a Kafka message consumer
			messages := processors.ConsumeAMQPMessages("", "")
			// Process each message
			for msg := range messages {
				ProcessAMQPMessage(msg)
			}
		default:
			log.Fatalf("Unsupported message queue type: %s", upstreamQueueType)
	}

	// Wait for termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

// Simulating message processing
func ProcessAMQPMessage(msg []byte) {
	// Simulating processing time
	time.Sleep(2 * time.Second)
	log.Printf("Processed message: %s\n", string(msg))
}

// Simulating message processing
func ProcessKafkaMessage(msg *kafka.Message) {
	// Simulating processing time
	time.Sleep(2 * time.Second)
	log.Printf("Processed message: %s\n", string(msg.Value))
}