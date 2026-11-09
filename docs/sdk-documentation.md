# SDK Documentation Standards

Every exported package, type, field whose meaning is not self-evident,
function, method, and constant must have a Go doc comment. Begin the comment
with the exact exported identifier and describe the contract rather than
repeating its name. Write complete sentences, keep the first paragraph useful
in isolation, and document concurrency, ownership, cancellation, retry,
ordering, and error semantics where they matter.

Use package comments to state scope and intended import path. Link related
symbols with `[Pipeline]` syntax, use paragraphs for rules, and use indented
examples only when they add information that an `Example...` test cannot.
Avoid implementation history, marketing language, and comments that restate a
field type. Keep configuration defaults in the type comment and validation
rules in `Validate` documentation.

```go
// PipelineConfig bounds concurrent and queued unacknowledged deliveries.
//
// Workers is the maximum number of simultaneous sink deliveries. Buffer is
// the number of additional deliveries retained in memory; together they bound
// unacknowledged work. MessageTimeout limits one delivery including retry
// waits. ShutdownTimeout limits graceful draining after [Pipeline.Shutdown].
//
// All timeouts must be positive. A zero Buffer is valid and applies immediate
// backpressure to the source.
type PipelineConfig struct {
    Workers         int
    Buffer          int
    MessageTimeout  time.Duration
    ShutdownTimeout time.Duration
}
```

Examples belong in `example_test.go` and use the external `bridge_test`
package unless an unexported detail is intentionally being demonstrated.
Name examples after the symbol (`ExamplePipeline_Run`) and include exact
`// Output:` only for deterministic output. Examples must neither require a
network service nor hide errors; use a small in-memory source and sink instead.

Run `go doc ./pkg/bridge/...`, `go test ./...`, and `golangci-lint run` before
reviewing documentation changes. The linter configuration enforces exported
API comments through `revive` and catches ignored errors, unsafe contexts,
resource leaks, SQL close errors, and common concurrency mistakes.
