# Test scaffolding

## Fast unit tests

Run the default suite without Docker or external services:

```sh
go test ./...
```

`internal/testkit` supplies deterministic, context-aware `Source`, `Sink`, and
`Delivery` fakes. `Source.Pause` blocks intake, `Sink.SetGate` creates
backpressure, and `FailNext` scripts FIFO transient failures. These controls
make concurrency and retry tests independent of wall-clock polling.

## Opt-in suites

Existing broker and database tests are intentionally excluded from default
test runs:

```sh
go test -tags integration ./test/integration
```

They require the relevant service environment variables and may be run against
the Docker Compose stack when available. The dependency-free chaos skeleton
exercises fault-injection contracts locally:

```sh
go test -tags chaos ./test/chaos
```

Add a chaos scenario only when it has a bounded timeout, honors context
cancellation, and leaves no process or external resource behind.
