// Package cdk contains helpers for implementing external bridge connectors.
package cdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/registry"
)

// ConfigValidator validates decoded connector configuration.
//
// Implement Validate on either the config value or its pointer type.
type ConfigValidator interface {
	Validate() error
}

// DecodeConfig strictly decodes a connector configuration and calls Validate
// when the decoded value implements ConfigValidator. Empty and null
// configuration are rejected.
func DecodeConfig[C any](raw json.RawMessage) (C, error) {
	var config C
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return config, errors.New("configuration is required")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("decode configuration: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return config, errors.New("decode configuration: multiple JSON values")
		}
		return config, fmt.Errorf("decode configuration: %w", err)
	}

	if validator, ok := any(config).(ConfigValidator); ok {
		if err := validator.Validate(); err != nil {
			return config, fmt.Errorf("validate configuration: %w", err)
		}
	} else if validator, ok := any(&config).(ConfigValidator); ok {
		if err := validator.Validate(); err != nil {
			return config, fmt.Errorf("validate configuration: %w", err)
		}
	}
	return config, nil
}

// AssertSource returns source as its bridge contract. It provides a concise
// compile-time conformance assertion for connector implementations.
func AssertSource[T any](source bridge.Source[T]) bridge.Source[T] {
	return source
}

// AssertSink returns sink as its bridge contract. It provides a concise
// compile-time conformance assertion for connector implementations.
func AssertSink[T any](sink bridge.Sink[T]) bridge.Sink[T] {
	return sink
}

// SourceBuilder constructs a raw-payload source from validated configuration.
type SourceBuilder[C any] func(context.Context, C, bridge.Dependencies) (bridge.Source[[]byte], error)

// SinkBuilder constructs a raw-payload sink from validated configuration.
type SinkBuilder[C any] func(context.Context, C, bridge.Dependencies) (bridge.Sink[[]byte], error)

// SourceFactory adapts a typed SourceBuilder to the registry factory contract.
// It decodes configuration and applies default dependencies, but does not
// register globally; applications choose when and where to register it.
func SourceFactory[C any](build SourceBuilder[C]) registry.SourceFactory {
	return func(ctx context.Context, raw json.RawMessage, deps bridge.Dependencies) (bridge.Source[[]byte], error) {
		if build == nil {
			return nil, errors.New("source builder is required")
		}
		config, err := DecodeConfig[C](raw)
		if err != nil {
			return nil, err
		}
		source, err := build(ctx, config, deps.Defaults())
		if err != nil {
			return nil, fmt.Errorf("build source: %w", err)
		}
		if source == nil {
			return nil, errors.New("source builder returned nil")
		}
		return source, nil
	}
}

// SinkFactory adapts a typed SinkBuilder to the registry factory contract.
// It decodes configuration and applies default dependencies, but does not
// register globally; applications choose when and where to register it.
func SinkFactory[C any](build SinkBuilder[C]) registry.SinkFactory {
	return func(ctx context.Context, raw json.RawMessage, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
		if build == nil {
			return nil, errors.New("sink builder is required")
		}
		config, err := DecodeConfig[C](raw)
		if err != nil {
			return nil, err
		}
		sink, err := build(ctx, config, deps.Defaults())
		if err != nil {
			return nil, fmt.Errorf("build sink: %w", err)
		}
		if sink == nil {
			return nil, errors.New("sink builder returned nil")
		}
		return sink, nil
	}
}
