// Package rabbitmq provides a RabbitMQ bridge source connector factory.
package rabbitmq

import (
	"context"

	"gostreambridge/connectors/adapter"
	"gostreambridge/connectors/config"
	"gostreambridge/internal/source"
	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
	"gostreambridge/pkg/bridge/registry"
)

// SourceFactory creates RabbitMQ sources from {"configDir":"..."}.
var SourceFactory = cdk.SourceFactory(func(ctx context.Context, cfg config.Config, _ bridge.Dependencies) (bridge.Source[[]byte], error) {
	legacy, err := source.NewRabbitMQSource(ctx, cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	return adapter.NewSource(legacy)
})

func init() {
	registry.RegisterSource("rabbitmq", SourceFactory)
}
