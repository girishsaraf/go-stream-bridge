// Package connector is a minimal external sink connector template.
package connector

import (
	"context"
	"errors"

	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/cdk"
)

// Config is the connector's JSON configuration.
type Config struct {
	Prefix string `json:"prefix"`
}

// Validate rejects unusable connector configuration.
func (c Config) Validate() error {
	if c.Prefix == "" {
		return errors.New("prefix is required")
	}
	return nil
}

// Factory is deliberately unregistered so it can be tested without mutating
// the process-wide registry.
var Factory = cdk.SinkFactory(New)

// New creates the connector from validated configuration.
func New(_ context.Context, config Config, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
	return &sink{prefix: config.Prefix, logger: deps.Logger}, nil
}

type sink struct {
	prefix string
	logger bridge.Logger
}

func (s *sink) Open(context.Context) error { return nil }

func (s *sink) Deliver(ctx context.Context, event bridge.Event[[]byte]) error {
	s.logger.Info(ctx, s.prefix+" received event", bridge.Field{Key: "event_id", Value: event.Metadata.ID})
	return nil
}

func (s *sink) Close(context.Context) error { return nil }

var _ bridge.Sink[[]byte] = cdk.AssertSink[[]byte]((*sink)(nil))
