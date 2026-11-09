// Package kafka provides Kafka bridge source and sink connector factories.
package kafka

import (
	"context"

	"gostreambridge/connectors/adapter"
	"gostreambridge/connectors/config"
	legacysink "gostreambridge/internal/sink"
	"gostreambridge/internal/source"
	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
	"gostreambridge/pkg/bridge/registry"
)

// SourceFactory creates Kafka sources from {"configDir":"..."}.
var SourceFactory = cdk.SourceFactory(func(_ context.Context, cfg config.Config, _ bridge.Dependencies) (bridge.Source[[]byte], error) {
	legacy, err := source.NewKafkaSource(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	return adapter.NewSource(legacy)
})

// SinkFactory creates Kafka sinks from {"configDir":"..."}.
var SinkFactory = cdk.SinkFactory(func(_ context.Context, cfg config.Config, _ bridge.Dependencies) (bridge.Sink[[]byte], error) {
	legacy, err := legacysink.NewKafkaSink(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	return adapter.NewSink(legacy)
})

func init() {
	registry.RegisterSource("kafka", SourceFactory)
	registry.RegisterSink("kafka", SinkFactory)
}
