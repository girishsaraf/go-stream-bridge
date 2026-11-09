// Package mysql provides a MySQL bridge sink connector factory.
package mysql

import (
	"context"

	"gostreambridge/connectors/adapter"
	"gostreambridge/connectors/config"
	legacysink "gostreambridge/internal/sink"
	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
	"gostreambridge/pkg/bridge/registry"
)

// SinkFactory creates MySQL sinks from {"configDir":"..."}.
var SinkFactory = cdk.SinkFactory(func(ctx context.Context, cfg config.Config, _ bridge.Dependencies) (bridge.Sink[[]byte], error) {
	legacy, err := legacysink.NewSQLSink(ctx, "mysql", cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	return adapter.NewSink(legacy)
})

func init() {
	registry.RegisterSink("mysql", SinkFactory)
}
