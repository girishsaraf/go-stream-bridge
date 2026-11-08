#!/usr/bin/env bash
set -euo pipefail

# Defaults match docker-compose.test.yml. Individual tests skip when their
# service environment is absent, so this script can be pointed at any stack.
export KAFKA_BROKERS="${KAFKA_BROKERS:-localhost:9092}"
export RABBITMQ_URL="${RABBITMQ_URL:-amqp://guest:guest@localhost:5672/}"
export MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
export MYSQL_PORT="${MYSQL_PORT:-3307}"
export MYSQL_USER="${MYSQL_USER:-stream}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-stream}"
export MYSQL_DATABASE="${MYSQL_DATABASE:-stream_bridge}"
export ELASTICSEARCH_URL="${ELASTICSEARCH_URL:-http://localhost:9200}"

# SQL Server is optional and normally only enabled through the sqlserver
# Compose profile.
if [[ -n "${SQLSERVER_HOST:-}" ]]; then
  export SQLSERVER_HOST SQLSERVER_PORT="${SQLSERVER_PORT:-1433}"
  export SQLSERVER_USER="${SQLSERVER_USER:-sa}"
  export SQLSERVER_PASSWORD="${SQLSERVER_PASSWORD:-Str0ng!Passw0rd}"
  export SQLSERVER_DATABASE="${SQLSERVER_DATABASE:-stream_bridge}"
fi

go test -tags=integration -count=1 -timeout=20m -v ./test/integration/...
