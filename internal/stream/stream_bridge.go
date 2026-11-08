package stream

import (
	"context"
	"fmt"
)

// StreamBridge defines the interface for a message stream bridge
type StreamBridge interface {
	ConsumeMessages(ctx context.Context, downstreamApp string) error
}

// StartStreamBridge starts the stream bridge based on provided details. It
// blocks until the context is cancelled or the upstream consumer fails.
func StartStreamBridge(ctx context.Context, bridgeDetails map[string]string) error {
	upstreamApp := bridgeDetails["upstreamApp"]
	downstreamApp := bridgeDetails["downstreamApp"]

	var stream StreamBridge
	switch upstreamApp {
	case "kafka":
		stream = KafkaStreamBridge{}
	case "rabbitmq":
		stream = RabbitMQStreamBridge{}
	default:
		return fmt.Errorf("unsupported upstream app: %s", upstreamApp)
	}

	if err := ValidateDownstream(downstreamApp); err != nil {
		return err
	}

	return stream.ConsumeMessages(ctx, downstreamApp)
}
