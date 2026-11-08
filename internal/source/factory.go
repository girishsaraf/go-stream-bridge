package source

import (
	"context"
	"fmt"
)

// New creates the source for the named upstream application.
func New(ctx context.Context, name, configDir string) (Source, error) {
	switch name {
	case "kafka":
		return NewKafkaSource(configDir)
	case "rabbitmq":
		return NewRabbitMQSource(ctx, configDir)
	default:
		return nil, fmt.Errorf("unsupported upstream app: %s", name)
	}
}
