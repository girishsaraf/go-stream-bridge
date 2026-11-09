// Package bridge defines the public contracts for building event pipelines.
package bridge

import (
	"context"
	"time"
)

// Metadata describes a delivery without coupling the pipeline to a transport.
// Connectors must treat fields supplied by the source as immutable.
type Metadata struct {
	ID          string
	Source      string
	Topic       string
	Partition   int32
	Offset      int64
	Key         []byte
	Headers     map[string][]byte
	ReceivedAt  time.Time
	Attempt     int
	TraceParent string
	Schema      SchemaReference
}

// SchemaReference identifies the schema used to encode an event payload.
type SchemaReference struct {
	Subject string
	ID      string
	Version string
	Format  string
}

// Event is the payload and metadata processed by a pipeline.
type Event[T any] struct {
	Payload  T
	Metadata Metadata
}

// Delivery is a source-owned event that must complete exactly once. Ack
// confirms a terminal outcome; Nack leaves the event eligible for redelivery.
type Delivery[T any] interface {
	Event() Event[T]
	Ack(context.Context) error
	Nack(context.Context, error) error
}

// Source supplies deliveries until the context is cancelled or it returns an
// error. Pause must stop new intake while allowing outstanding deliveries to
// complete.
type Source[T any] interface {
	Open(context.Context) error
	Receive(context.Context) (Delivery[T], error)
	Pause(context.Context) error
	Resume(context.Context) error
	Close(context.Context) error
}

// Sink persists or publishes an event.
type Sink[T any] interface {
	Open(context.Context) error
	Deliver(context.Context, Event[T]) error
	Close(context.Context) error
}

// Engine owns a pipeline's run and shutdown lifecycle.
type Engine[T any] interface {
	Run(context.Context) error
	Shutdown(context.Context) error
}

// Codec translates raw transport events to and from an application type.
type Codec[T any] interface {
	Decode(context.Context, Event[[]byte]) (Event[T], error)
	Encode(context.Context, Event[T]) (Event[[]byte], error)
}
