package bridge

import "context"

// Schema is an immutable schema definition resolved from a registry.
type Schema interface {
	Reference() SchemaReference
}

// Registry resolves schemas and verifies compatibility.
type Registry interface {
	Resolve(context.Context, SchemaReference) (Schema, error)
	CheckCompatibility(context.Context, SchemaReference, Schema) error
}

// FormatCodec converts application values using a resolved schema.
type FormatCodec[T any] interface {
	Decode(context.Context, Schema, []byte) (T, error)
	Encode(context.Context, Schema, T) ([]byte, error)
}
