package stream

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// StreamBridge defines the interface for a message stream bridge
type StreamBridge interface {
	ConsumeMessages(bridgeDetails map[string]string)
}

// StartStreamBridge starts the stream bridge based on provided details
func StartStreamBridge(bridgeDetails map[string]string) {
	var stream StreamBridge

	switch bridgeDetails["upstreamApp"] {
	case "kafka":
		stream = KafkaStreamBridge{}
	case "rabbitmq":

		stream = RabbitMQStreamBridge{}
	default:
		log.Fatalf("Unsupported message queue type: %s", bridgeDetails["upstreamApp"])
	}

	go stream.ConsumeMessages(bridgeDetails)

	// Wait for termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
