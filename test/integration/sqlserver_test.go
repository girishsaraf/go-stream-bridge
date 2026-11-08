//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

type sqlServerSettings struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func sqlServerEnv(t *testing.T) sqlServerSettings {
	t.Helper()

	host := os.Getenv("SQLSERVER_HOST")
	if host == "" {
		t.Skip("SQLSERVER_HOST is not set; skipping SQL Server integration test")
	}
	return sqlServerSettings{
		Host:     host,
		Port:     envOrDefault("SQLSERVER_PORT", "1433"),
		User:     envOrDefault("SQLSERVER_USER", "sa"),
		Password: envOrDefault("SQLSERVER_PASSWORD", "Str0ng!Passw0rd"),
		Database: envOrDefault("SQLSERVER_DATABASE", "stream_bridge"),
	}
}

func (s sqlServerSettings) config() map[string]any {
	return mysqlConfig(s.Host, s.Port, s.User, s.Password, s.Database)
}

func (s sqlServerSettings) dsn() string {
	query := url.Values{}
	query.Set("database", s.Database)
	query.Set("encrypt", "disable")
	return (&url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(s.User, s.Password),
		Host:     s.Host + ":" + s.Port,
		RawQuery: query.Encode(),
	}).String()
}

func openSQLServer(t *testing.T, settings sqlServerSettings) *sql.DB {
	t.Helper()

	ensureSQLServerDatabase(t, settings)

	db, err := sql.Open("sqlserver", settings.dsn())
	if err != nil {
		t.Fatalf("open sqlserver: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping sqlserver: %v", err)
	}
	return db
}

// ensureSQLServerDatabase creates the target database through a master
// connection, since the SQL Server image does not create application databases.
func ensureSQLServerDatabase(t *testing.T, settings sqlServerSettings) {
	t.Helper()

	master := settings
	master.Database = "master"

	db, err := sql.Open("sqlserver", master.dsn())
	if err != nil {
		t.Fatalf("open sqlserver master: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping sqlserver master: %v", err)
	}

	const createDatabase = `
IF DB_ID(@p1) IS NULL
BEGIN
    DECLARE @statement nvarchar(max) = N'CREATE DATABASE ' + QUOTENAME(@p1);
    EXEC (@statement);
END`
	if _, err := db.ExecContext(ctx, createDatabase, settings.Database); err != nil {
		t.Fatalf("create sqlserver database %q: %v", settings.Database, err)
	}
}

func applySQLServerSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	schema, err := os.ReadFile(filepath.Join("..", "..", "internal", "config", "files", "sqlserver.sql"))
	if err != nil {
		t.Fatalf("read sqlserver schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply sqlserver schema: %v", err)
	}
}

func TestKafkaToSQLServer(t *testing.T) {
	brokers := kafkaBrokers(t)
	settings := sqlServerEnv(t)
	db := openSQLServer(t, settings)
	applySQLServerSchema(t, db)
	if _, err := db.Exec("DELETE FROM messages"); err != nil {
		t.Fatalf("truncate sqlserver messages: %v", err)
	}

	inputTopic := uniqueName("sqlserver-input")
	group := uniqueName("sqlserver-group")

	var values []string
	for i := 0; i < 5; i++ {
		values = append(values, fmt.Sprintf("sqlserver-%02d", i))
	}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"sqlserver.json":      settings.config(),
		"bridge.json":         bridgeConfig(2, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "kafka", "sqlserver")
	waitFor(t, 30*time.Second, 250*time.Millisecond, "sqlserver row count", func() (bool, string) {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil {
			return false, err.Error()
		}
		return count == len(values), fmt.Sprintf("rows = %d", count)
	})

	rows, err := db.Query("SELECT message FROM messages ORDER BY id")
	if err != nil {
		t.Fatalf("query sqlserver messages: %v", err)
	}
	var messages []string
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			_ = rows.Close()
			t.Fatalf("scan sqlserver message: %v", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close sqlserver rows: %v", err)
	}
	requireSameMessages(t, values, messages)

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, int64(len(values)))
}
