package stream

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"gostreambridge/internal/database/dsprocessors"
	"gostreambridge/internal/queue/processors"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func StartStreamBridge(bridgeDetails map[string]string) {
	ConsumeMessages(bridgeDetails)
}

func ConsumeMessages(bridgeDetails map[string]string) {

	switch bridgeDetails["upstreamApp"] {
	case "kafka":
		// Create a Kafka message consumer
		messages := processors.ConsumeKafkaMessages()
		// Process each message
		for msg := range messages {
			ProcessKafkaMessage(bridgeDetails["downstreamApp"], msg)
		}
	case "rabbitmq":
		// Create a Kafka message consumer
		messages := processors.ConsumeAMQPMessages()
		// Process each message
		for msg := range messages {
			ProcessAMQPMessage(bridgeDetails["downstreamApp"], msg)
		}
	default:
		log.Fatalf("Unsupported message queue type: %s", bridgeDetails["upstreamApp"])
	}

	// Wait for termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

// ProcessAMQPMessage Simulating message processing
func ProcessAMQPMessage(downstreamAppName string, msg []byte) {
	// Calling the appropriate downstream flow
	message := string(msg)
	err := DetermineDownstreamFlow(downstreamAppName, message)
	if err != nil {
		return
	}
	log.Printf("Processed message: %s\n", string(msg))
}

// ProcessKafkaMessage Simulating message processing
func ProcessKafkaMessage(downstreamAppName string, msg *kafka.Message) {
	// Calling the appropriate downstream flow
	err := DetermineDownstreamFlow(downstreamAppName, string(msg.Value))
	if err != nil {
		return
	}
	log.Printf("Processed message: %s\n", string(msg.Value))
}

// DetermineDownstreamFlow to determine which function to call downstream
func DetermineDownstreamFlow(downstreamAppName string, message string) error {
	switch downstreamAppName {
	case "mysql":
		err := dsprocessors.WriteToMySQL(message)
		if err != nil {
			return err
		}
		return nil
	case "sqlserver":
		err := dsprocessors.WriteToSQLServer(message)
		if err != nil {
			return err
		}
		return nil
	case "elastic":
		err := dsprocessors.WriteToElastic(message)
		if err != nil {
			return err
		}
		return nil
	case "kafka":
		err := processors.WriteToKafka(message)
		if err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}
