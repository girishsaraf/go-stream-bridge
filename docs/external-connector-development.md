# External connector development

External connectors import the public contracts from
`gostreambridge/pkg/bridge` and the helpers from
`gostreambridge/pkg/bridge/cdk`. Implement sources and sinks with raw
payloads (`[]byte`) so the bridge can apply codecs and pipeline behavior.

## Build a connector

Define a typed JSON configuration with validation, then create a factory:

```go
type Config struct {
    URL string `json:"url"`
}

func (c Config) Validate() error {
    if c.URL == "" {
        return errors.New("url is required")
    }
    return nil
}

func New(ctx context.Context, cfg Config, deps bridge.Dependencies) (bridge.Sink[[]byte], error) {
    return &sink{/* initialize one client here */}, nil
}

var Factory = cdk.SinkFactory(New)
var _ bridge.Sink[[]byte] = cdk.AssertSink[[]byte]((*sink)(nil))
```

`cdk.DecodeConfig`, used by `SinkFactory` and `SourceFactory`, rejects empty
configuration, unknown fields, malformed JSON, and trailing JSON values. It
then invokes `Validate` when the config implements `cdk.ConfigValidator`.
Factories also receive dependencies with no-op observability defaults applied.

Implement all `bridge.Source` methods for sources, including `Pause` and
`Resume`. `Receive` should return `io.EOF` for a clean finite source
completion. A source owns each delivery and must make `Ack` and `Nack`
complete its transport-specific terminal operation exactly once. Sinks should
create reusable clients in `Open`, deliver one event per `Deliver`, and release
resources in `Close`.

## Register from the application

Keep connector packages free of `init` registration. Expose a factory and let
the application that assembles connectors register it once:

```go
registry.RegisterSink("acme-webhook", acme.Factory)
registry.RegisterSource("acme-events", acme.SourceFactory)
```

This avoids global registry state during unit tests. Test `Factory` directly
with JSON and `bridge.Dependencies{}`; no registration is required. The
runnable template at `examples/cdk/connector` and
`examples/cdk/register` follows this pattern.

## Delivery rules

The bridge provides at-least-once delivery. Treat metadata supplied by sources
as immutable and make sinks idempotent where duplicate writes are unsafe.
Connector methods must honor context cancellation. Return retryable transport
errors from `Deliver`; retry policy and terminal failure handling belong to the
pipeline, not the connector.
