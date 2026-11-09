// Package registry discovers compiled source and sink connector factories.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gostreambridge/pkg/bridge"
)

// SourceFactory creates a raw-payload source from connector-specific config.
type SourceFactory func(context.Context, json.RawMessage, bridge.Dependencies) (bridge.Source[[]byte], error)

// SinkFactory creates a raw-payload sink from connector-specific config.
type SinkFactory func(context.Context, json.RawMessage, bridge.Dependencies) (bridge.Sink[[]byte], error)

var factories = struct {
	sync.RWMutex
	sources map[string]SourceFactory
	sinks   map[string]SinkFactory
}{
	sources: make(map[string]SourceFactory),
	sinks:   make(map[string]SinkFactory),
}

// RegisterSource registers a compiled source connector. It panics on invalid
// input or duplicate names so registration failures in init functions are
// visible during application startup.
func RegisterSource(name string, factory SourceFactory) {
	registerSource(normalizeName(name), factory)
}

func registerSource(name string, factory SourceFactory) {
	if name == "" {
		panic("bridge registry: source name is empty")
	}
	if factory == nil {
		panic(fmt.Sprintf("bridge registry: source %q factory is nil", name))
	}

	factories.Lock()
	defer factories.Unlock()
	if _, exists := factories.sources[name]; exists {
		panic(fmt.Sprintf("bridge registry: source %q registered twice", name))
	}
	factories.sources[name] = factory
}

// RegisterSink registers a compiled sink connector. It panics on invalid input
// or duplicate names so registration failures in init functions are visible
// during application startup.
func RegisterSink(name string, factory SinkFactory) {
	registerSink(normalizeName(name), factory)
}

func registerSink(name string, factory SinkFactory) {
	if name == "" {
		panic("bridge registry: sink name is empty")
	}
	if factory == nil {
		panic(fmt.Sprintf("bridge registry: sink %q factory is nil", name))
	}

	factories.Lock()
	defer factories.Unlock()
	if _, exists := factories.sinks[name]; exists {
		panic(fmt.Sprintf("bridge registry: sink %q registered twice", name))
	}
	factories.sinks[name] = factory
}

// NewSource constructs a registered source connector.
func NewSource(ctx context.Context, name string, config json.RawMessage, deps bridge.Dependencies) (bridge.Source[[]byte], error) {
	name = normalizeName(name)
	factories.RLock()
	factory, ok := factories.sources[name]
	factories.RUnlock()
	if !ok {
		return nil, fmt.Errorf("bridge registry: source %q is not registered", name)
	}
	return factory(ctx, config, deps.Defaults())
}

// NewSink constructs a registered sink connector.
func NewSink(ctx context.Context, name string, config json.RawMessage, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
	name = normalizeName(name)
	factories.RLock()
	factory, ok := factories.sinks[name]
	factories.RUnlock()
	if !ok {
		return nil, fmt.Errorf("bridge registry: sink %q is not registered", name)
	}
	return factory(ctx, config, deps.Defaults())
}

// Sources returns registered source connector names in sorted order.
func Sources() []string {
	factories.RLock()
	defer factories.RUnlock()
	return sortedKeys(factories.sources)
}

// Sinks returns registered sink connector names in sorted order.
func Sinks() []string {
	factories.RLock()
	defer factories.RUnlock()
	return sortedKeys(factories.sinks)
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func sortedKeys[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
