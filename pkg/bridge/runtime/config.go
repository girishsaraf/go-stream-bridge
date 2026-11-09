// Package runtime builds a single raw-payload bridge pipeline from a portable
// JSON configuration document.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/registry"
)

// Connector selects a registered connector and provides connector-owned JSON
// configuration.
type Connector struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// Config describes one source-to-sink reference pipeline.
type Config struct {
	Source   Connector             `json:"source"`
	Sink     Connector             `json:"sink"`
	Pipeline bridge.PipelineConfig `json:"pipeline"`
}

// Validate checks the runtime-owned portion of a pipeline configuration.
func (c Config) Validate() error {
	if c.Source.Type == "" {
		return errors.New("source type is required")
	}
	if c.Sink.Type == "" {
		return errors.New("sink type is required")
	}
	if err := c.Pipeline.Validate(); err != nil {
		return fmt.Errorf("pipeline: %w", err)
	}
	return nil
}

// Load decodes and validates a reference runtime configuration.
func Load(data []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("decode runtime config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// Build constructs a registered source and sink pipeline.
func Build(ctx context.Context, config Config, deps bridge.Dependencies, options ...bridge.PipelineOption[[]byte]) (*bridge.Pipeline[[]byte], error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	source, err := registry.NewSource(ctx, config.Source.Type, config.Source.Config, deps)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}
	sink, err := registry.NewSink(ctx, config.Sink.Type, config.Sink.Config, deps)
	if err != nil {
		return nil, fmt.Errorf("create sink: %w", err)
	}
	pipeline, err := bridge.NewPipeline(source, sink, config.Pipeline, nil, options...)
	if err != nil {
		closeErr := source.Close(ctx)
		return nil, errors.Join(err, closeErr)
	}
	return pipeline, nil
}
