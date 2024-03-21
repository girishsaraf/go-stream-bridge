package stream

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// StreamBridge defines the interface for a message stream bridge
type StreamBridge interface {
	ConsumeMessages(bridgeDetails map[string]string)
}

// StartStreamBridge starts the stream bridge based on provided details
func StartStreamBridge(bridgeDetails map[string]string) {
	zerolog.New(os.Stdout)

	var stream StreamBridge

	switch bridgeDetails["upstreamApp"] {
	case "kafka":
		stream = KafkaStreamBridge{}
	case "rabbitmq":
		stream = RabbitMQStreamBridge{}
	default:
		log.Fatal().Msgf("Unsupported message queue type: %s", bridgeDetails["upstreamApp"])
	}

	go stream.ConsumeMessages(bridgeDetails)

	// Wait for termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
