// Package elasticsearch provides an Elasticsearch bridge sink connector factory.
package elasticsearch

import (
	"context"

	"gostreambridge/connectors/adapter"
	"gostreambridge/connectors/config"
	legacysink "gostreambridge/internal/sink"
	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
	"gostreambridge/pkg/bridge/registry"
)

// SinkFactory creates Elasticsearch sinks from {"configDir":"..."}.
var SinkFactory = cdk.SinkFactory(func(ctx context.Context, cfg config.Config, _ bridge.Dependencies) (bridge.Sink[[]byte], error) {
	legacy, err := legacysink.NewElasticSink(ctx, cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	return adapter.NewSink(legacy)
})

func init() {
	registry.RegisterSink("elasticsearch", SinkFactory)
}
