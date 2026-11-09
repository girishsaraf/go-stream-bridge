# go-stream-bridge

`go-stream-bridge` moves messages from an upstream broker to a downstream datastore with at-least-once delivery, bounded concurrency, centralized retries, and optional dead-lettering.

| Upstream | Downstream |
| --- | --- |
| Kafka | Kafka, MySQL, SQL Server, Elasticsearch |
| RabbitMQ | Kafka, MySQL, SQL Server, Elasticsearch |

Clients and connection pools are created once per process and reused for every message. Long-lived resources are closed during graceful shutdown.

## Features

- Ack-aware consumption: a message is acknowledged or committed only after a terminal outcome.
- Kafka contiguous-offset tracking that is safe with concurrent workers, rebalances, and compacted gaps.
- Bounded worker pool with backpressure from the downstream sink.
- Pipeline-level exponential backoff with permanent-error classification.
- `stop` (fail fast) or `dlq` (dead-letter and continue) failure policies.
- Kafka and durable JSONL file dead-letter writers.
- `${VAR}` environment expansion in JSON configuration with fail-fast on unset variables.
- Structured console or JSON logging, signal-driven graceful shutdown, and build version metadata.

## SDK architecture

The public SDK separates transport adapters from the bounded processing core.
Applications enable compiled connectors with blank imports or explicit
registration; third-party connectors follow the [Connector Developer
Kit](docs/external-connector-development.md).

```mermaid
flowchart LR
    S[Source connector\nKafka / RabbitMQ / SQS] --> I[Bounded intake]
    I --> M[Middleware\nvalidate / enrich / audit]
    M --> R[Router and schema codec]
    R --> B[Worker pool / batcher]
    B --> K[Sink connector\nSQL / broker / object storage]
    B -->|terminal failure| D[DLQ]
    O[Logger / meter / tracer] -.-> I
    O -.-> B
```

Each delivery remains unacknowledged until every required sink succeeds or a
durable DLQ handler accepts its terminal failure. A full queue applies
backpressure instead of accumulating unbounded memory.

## SDK quick start: Kafka to PostgreSQL

This ten-line bootstrap assumes a PostgreSQL connector built with the CDK is
registered as `postgres`; the repository's built-in SQL adapters currently
cover MySQL and SQL Server.

```go
ctx := context.Background()
cfg, err := runtime.Load([]byte(`{"source":{"type":"kafka","config":{"brokers":"localhost:9092","topic":"orders","groupId":"pg-bridge"}},"sink":{"type":"postgres","config":{"dsn":"postgres://bridge:secret@localhost/orders"}},"pipeline":{"workers":4,"buffer":256,"messageTimeout":30000000000,"shutdownTimeout":30000000000}}`))
if err != nil { log.Fatal(err) }
pipeline, err := runtime.Build(ctx, cfg, bridge.Dependencies{})
if err != nil { log.Fatal(err) }
if err := pipeline.Run(ctx); err != nil { log.Fatal(err) }
```

Import the Kafka and PostgreSQL connector packages in the application that owns
this bootstrap. See [`pkg/bridge/example_test.go`](pkg/bridge/example_test.go)
for a runnable, infrastructure-free SDK example.

### Configuration matrix

| Setting | Default | Environment override | Notes |
| --- | --- | --- | --- |
| Source/sink connector | none | application-owned | Registered connector type and JSON configuration. |
| `workers` | `1` | none | Legacy CLI default; SDK `PipelineConfig` requires an explicit positive value. |
| `buffer` | `256` | none | Legacy CLI default; bounds queued unacknowledged deliveries. |
| `messageTimeout` | `30s` | none | Legacy CLI default; includes retries and DLQ writes. |
| `shutdownTimeout` | `30s` | none | Legacy CLI graceful-drain deadline. |
| `retry.attempts` | `5` | none | Total attempts, including the first. |
| `retry.baseDelay` / `maxDelay` | `2s` / `30s` | none | Exponential retry bounds. |
| Config directory | `internal/config/files` | `STREAM_BRIDGE_CONFIG_DIR` | Legacy CLI configuration directory. |
| Logging level | `info` | `LOG_LEVEL` | `trace` through `panic`. |
| Logging format | console | `LOG_FORMAT` | Set to `json` for structured output. |

## Documentation and decisions

- [SDK documentation standards](docs/sdk-documentation.md) define exported API
  comments and executable `Example...` tests.
- [External connector development](docs/external-connector-development.md)
  documents the CDK and conformance expectations.
- [ADR template](docs/adr/template.md) records durable architectural decisions.
- `.golangci.yml` enforces documentation, error handling, context, resource,
  concurrency, and formatting checks in CI.

## Requirements

- Go 1.27.3 or newer.
- CGO and the system libraries required by `confluent-kafka-go` (`libssl`, `libsasl2`, `zlib`, `libzstd`, `liblz4`, `libcurl`). The provided Dockerfile installs them.

## Quick start

```sh
make build

# Edit internal/config/files/*.json, then:
./bin/stream-bridge -upstreamApp=kafka -downstreamApp=mysql
```

Use a different configuration directory with `-configDir=/path/to/config` or `STREAM_BRIDGE_CONFIG_DIR`.

```sh
./bin/stream-bridge -version
./bin/stream-bridge -upstreamApp=rabbitmq -downstreamApp=kafka -configDir=/etc/stream-bridge
```

## Configuration

Each component reads one JSON file from the configuration directory:

| File | Used by | Required |
| --- | --- | --- |
| `kafka_consumer.json` | Kafka upstream | yes, for Kafka sources |
| `kafka_producer.json` | Kafka downstream | yes, for Kafka sinks |
| `rabbitmq.json` | RabbitMQ upstream | yes, for RabbitMQ sources |
| `mysql.json` | MySQL downstream | yes, for MySQL sinks |
| `sqlserver.json` | SQL Server downstream | yes, for SQL Server sinks |
| `elastic.json` | Elasticsearch downstream | yes, for Elasticsearch sinks |
| `bridge.json` | Runtime tuning | no; built-in defaults are used |
| `mysql.sql`, `sqlserver.sql` | Reference schemas | not read at runtime |

The checked-in files are local development examples. Replace hosts, topics, queues, and credentials for your environment.

### Kafka

```json
{
  "brokers": "localhost:9092",
  "topic": "topic1",
  "groupId": "stream-bridge"
}
```

`groupId` is required for consumers and ignored by producers. Consumers use `auto.offset.reset=earliest` and manual commits. Producers use `acks=all` and idempotence.

### RabbitMQ

```json
{
  "url": "amqp://guest:guest@localhost:5672",
  "queue": "rabbitmq_queue",
  "prefetch": 10,
  "durable": true,
  "autoDelete": false
}
```

`prefetch` defaults to 10. Queues are durable by default because RabbitMQ 4 rejects transient non-exclusive queues unless the broker explicitly enables them. Set `durable` to `false` only when the queue is intentionally transient and the broker permits it. Queue declarations must match the existing queue's durability and auto-delete settings or the broker rejects the declaration.

### Databases

```json
{
  "username": "mysql_user",
  "password": "mysql_password",
  "host": "localhost",
  "port": "3306",
  "database": "my_database"
}
```

MySQL and SQL Server use a prepared `INSERT INTO messages (message, timestamp) VALUES (?, ?)` / `(@p1, @p2)` statement and a connection pool (10 open connections, 5 idle, 30-minute lifetime). Apply `mysql.sql` or `sqlserver.sql` first.

### Elasticsearch

```json
{
  "url": "http://localhost:9200",
  "username": "",
  "password": "",
  "index": "messages"
}
```

The sink verifies connectivity at startup and indexes one document per message with `message` and `timestamp` fields.

### Runtime tuning (`bridge.json`)

`bridge.json` is optional. Fields that are omitted keep their defaults, including fields inside nested objects.

```json
{
  "workers": 1,
  "buffer": 256,
  "messageTimeout": "30s",
  "shutdownTimeout": "30s",
  "failurePolicy": "stop",
  "retry": {
    "attempts": 5,
    "baseDelay": "2s",
    "maxDelay": "30s"
  },
  "dlq": {
    "type": ""
  }
}
```

| Field | Default | Description |
| --- | --- | --- |
| `workers` | `1` | Concurrent message handlers. `1` preserves per-partition/queue order. |
| `buffer` | `256` | Bounded queue between the source and workers; provides backpressure. |
| `messageTimeout` | `30s` | Total budget for one message, including all retry attempts and DLQ writes. |
| `shutdownTimeout` | `30s` | How long to wait for in-flight messages after shutdown starts. |
| `failurePolicy` | `stop` | `stop` terminates the bridge on a terminal failure; `dlq` dead-letters and continues. |
| `retry.attempts` | `5` | Total attempts, including the first one. |
| `retry.baseDelay` | `2s` | Initial exponential backoff delay. |
| `retry.maxDelay` | `30s` | Backoff ceiling. |
| `dlq.type` | disabled | `kafka` or `file`. |

Durations must be strings with units (`"30s"`, `"2m"`, `"500ms"`); bare numbers are rejected.
Unknown configuration fields are rejected at startup, including fields nested inside
`retry` and `dlq`, so misspelled deployment settings fail fast.

### Secrets and environment expansion

Configuration files are expanded before JSON parsing. `${NAME}` is replaced with the environment value, `$$` produces a literal `$`, and an unset variable aborts startup instead of silently using an empty credential. Values are JSON-escaped, so quotes and backslashes in secrets are safe.

```json
{
  "username": "${MYSQL_USER}",
  "password": "${MYSQL_PASSWORD}",
  "host": "${MYSQL_HOST}",
  "port": "3306",
  "database": "stream_bridge"
}
```

Keep literal development values in the repository and inject production credentials through the environment or a secret manager.

### Logging and version

| Variable | Default | Description |
| --- | --- | --- |
| `STREAM_BRIDGE_CONFIG_DIR` | `internal/config/files` | Configuration directory (overridden by `-configDir`). |
| `LOG_LEVEL` | `info` | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. |
| `LOG_FORMAT` | console | Set to `json` for structured logs. |

Build metadata is injected with `-ldflags`; `make build` derives the version from `git describe`.

## Delivery semantics

### Acknowledgements and commits

- **Kafka**: offsets are tracked per partition and committed only at the highest contiguous successfully processed position. Commits run every 500 ms, on partition revocation, and on shutdown. Failed messages are left uncommitted and replay after restart.
- **RabbitMQ**: a delivery is acknowledged only after a terminal success or a successful DLQ write. Unacknowledged deliveries are requeued by the broker when the channel closes.
- Delivery is **at-least-once**. A crash between a downstream write and the commit/ack can duplicate a message. Make sinks idempotent if duplicates are unacceptable.

### Concurrency and ordering

The default `workers: 1` processes one message at a time, preserving per-partition and per-queue order. Increasing `workers` raises throughput but does not preserve global order; with Kafka, per-partition ordering is no longer guaranteed.

The `buffer` queue bounds how far the source can run ahead of the workers. When it fills, consumption blocks, which also stops offset commits and prefetching.

### Retries

Retries live in the pipeline, not in individual sinks. Each sink write is a single attempt; retryable failures are retried with exponential backoff up to `retry.attempts`, and the whole sequence is bounded by `messageTimeout`.

Errors classified as permanent fail immediately without retries. Examples include invalid Kafka messages, oversized records, authorization failures, SQL schema/constraint violations, and Elasticsearch 4xx responses other than 408 and 429. Deadlocks, timeouts, connection failures, and 5xx responses remain retryable.

### Failure policies

With `failurePolicy: "stop"` (the default), the first terminal failure cancels the pipeline, the failed message stays unacknowledged, and the process exits with an error. This is the safest setting when ordering or strict delivery matters.

With `failurePolicy: "dlq"`, exhausted or permanent failures are written to the configured dead-letter destination and the source message is then acknowledged:

```json
{
  "failurePolicy": "dlq",
  "dlq": {
    "type": "kafka",
    "brokers": "localhost:9092",
    "topic": "stream-bridge-dlq"
  }
}
```

```json
{
  "failurePolicy": "dlq",
  "dlq": {
    "type": "file",
    "path": "/var/lib/stream-bridge/dead-letters.jsonl"
  }
}
```

The file writer appends one JSON object per line, creates parent directories, restricts the file to `0600`, and `fsync`s every record. Each record contains the source, topic/partition/offset or queue/delivery tag, attempt count, error, timestamp, and base64-encoded payload.

If the DLQ write itself fails, the bridge stops and leaves the message unacknowledged so it is replayed rather than silently lost.

### Shutdown

`SIGINT` and `SIGTERM` cancel consumption, stop accepting new messages, and wait up to `shutdownTimeout` for in-flight work. Kafka commits the final contiguous positions; RabbitMQ acknowledges completed deliveries. Messages still in flight are left unacknowledged and replay after restart. If workers exceed the shutdown timeout, the process exits with an error and does not close resources they may still be using.

## Docker

```sh
make docker

# Mount a config directory whose files use ${VAR} references for secrets.
# Addresses in those files must be reachable from inside the container
# (for example kafka:9092 on a Compose network, not localhost).
docker run --rm \
  -e MYSQL_USER=stream \
  -e MYSQL_PASSWORD=secret \
  -v "$PWD/deploy/config:/config:ro" \
  go-stream-bridge:latest \
  -upstreamApp=kafka -downstreamApp=mysql -configDir=/config
```

The runtime image runs as an unprivileged user (`uid 10001`) and includes the native libraries required by the Kafka client.

## Testing

### Unit tests

```sh
make test-unit          # go test -race ./...
make lint               # go vet + staticcheck
make coverage
```

### Integration tests

Integration tests live in `test/integration` behind the `integration` build tag. They skip individually when their service environment variables are absent.

Start the core stack:

```sh
docker compose -f docker-compose.test.yml --profile core up -d --wait
make test-integration
docker compose -f docker-compose.test.yml --profile core down -v
```

`make test-integration` runs `scripts/test-integration.sh`, which supplies defaults matching the Compose stack and executes:

```sh
go test -tags=integration -count=1 -timeout=20m -v ./test/integration/...
```

| Variable | Default in script | Service |
| --- | --- | --- |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | RabbitMQ |
| `MYSQL_HOST` | `127.0.0.1` | MySQL |
| `MYSQL_PORT` | `3307` | MySQL |
| `MYSQL_USER` / `MYSQL_PASSWORD` | `stream` / `stream` | MySQL |
| `MYSQL_DATABASE` | `stream_bridge` | MySQL |
| `ELASTICSEARCH_URL` | `http://localhost:9200` | Elasticsearch |
| `SQLSERVER_HOST` | unset (test skipped) | SQL Server |
| `SQLSERVER_PORT` | `1433` | SQL Server |
| `SQLSERVER_USER` / `SQLSERVER_PASSWORD` | `sa` / `Str0ng!Passw0rd` | SQL Server |
| `SQLSERVER_DATABASE` | `stream_bridge` | SQL Server |

For SQL Server, start the optional profile and export `SQLSERVER_HOST`:

```sh
docker compose -f docker-compose.test.yml --profile sqlserver up -d --wait
SQLSERVER_HOST=127.0.0.1 make test-integration
```

Covered scenarios include Kafka-to-Kafka fan-out, multi-worker contiguous commits, restart replay, Kafka DLQ, Kafka-to-MySQL, MySQL DLQ, Kafka-to-Elasticsearch, RabbitMQ-to-Kafka, RabbitMQ DLQ/acknowledgement, SQL Server, and graceful shutdown.

CI (`.github/workflows/go.yml`) runs module verification, a committed-binary guard, build, vet, staticcheck, race-enabled unit tests, and the full Compose-backed integration suite, including SQL Server.

## Project layout

```
main.go                     CLI, logging, signals, version
internal/config             typed configs, env expansion, runtime validation
internal/source             Kafka and RabbitMQ sources, offset tracking
internal/sink               Kafka, SQL, and Elasticsearch sinks
internal/retry              context-aware exponential backoff
internal/dlq                Kafka and JSONL dead-letter writers
internal/stream             worker pool, retries, failure policy, shutdown
test/integration            build-tagged end-to-end tests
scripts/test-integration.sh integration test entry point
docker-compose.test.yml     Kafka, RabbitMQ, MySQL, Elasticsearch, SQL Server
```

## License

See [LICENSE](LICENSE).
