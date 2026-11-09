// Package sqlserver provides a SQL Server bridge sink connector factory.
package sqlserver

import (
	"context"

	"gostreambridge/connectors/adapter"
	"gostreambridge/connectors/config"
	legacysink "gostreambridge/internal/sink"
	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
	"gostreambridge/pkg/bridge/registry"
)

// SinkFactory creates SQL Server sinks from {"configDir":"..."}.
var SinkFactory = cdk.SinkFactory(func(ctx context.Context, cfg config.Config, _ bridge.Dependencies) (bridge.Sink[[]byte], error) {
	legacy, err := legacysink.NewSQLSink(ctx, "sqlserver", cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	return adapter.NewSink(legacy)
})

func init() {
	registry.RegisterSink("sqlserver", SinkFactory)
}
