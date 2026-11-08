package sink

import (
	"context"
	"fmt"
)

// New creates the sink for the named downstream application.
func New(ctx context.Context, name, configDir string) (Sink, error) {
	switch name {
	case "kafka":
		return NewKafkaSink(configDir)
	case "mysql":
		return NewSQLSink(ctx, "mysql", configDir)
	case "sqlserver":
		return NewSQLSink(ctx, "sqlserver", configDir)
	case "elastic":
		return NewElasticSink(ctx, configDir)
	default:
		return nil, fmt.Errorf("unsupported downstream app: %s", name)
	}
}
