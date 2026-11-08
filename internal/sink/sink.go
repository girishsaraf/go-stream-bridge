// Package sink provides long-lived downstream message writers.
package sink

import "context"

// Sink writes messages to a downstream system. Close releases all resources
// owned by the sink.
type Sink interface {
	Write(ctx context.Context, message []byte) error
	Close() error
}

var (
	_ Sink = (*KafkaSink)(nil)
	_ Sink = (*SQLSink)(nil)
	_ Sink = (*ElasticSink)(nil)
)
