//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type mysqlSettings struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func mysqlEnv(t *testing.T) mysqlSettings {
	t.Helper()
	return mysqlSettings{
		Host:     requireEnv(t, "MYSQL_HOST"),
		Port:     envOrDefault("MYSQL_PORT", "3306"),
		User:     envOrDefault("MYSQL_USER", "stream"),
		Password: envOrDefault("MYSQL_PASSWORD", "stream"),
		Database: envOrDefault("MYSQL_DATABASE", "stream_bridge"),
	}
}

func (s mysqlSettings) config() map[string]any {
	return mysqlConfig(s.Host, s.Port, s.User, s.Password, s.Database)
}

func (s mysqlSettings) dsn() string {
	cfg := mysql.NewConfig()
	cfg.User = s.User
	cfg.Passwd = s.Password
	cfg.Net = "tcp"
	cfg.Addr = s.Host + ":" + s.Port
	cfg.DBName = s.Database
	cfg.ParseTime = true
	return cfg.FormatDSN()
}

func openMySQL(t *testing.T, settings mysqlSettings) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", settings.dsn())
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping mysql: %v", err)
	}
	return db
}

func applyMySQLSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	schema, err := os.ReadFile(filepath.Join("..", "..", "internal", "config", "files", "mysql.sql"))
	if err != nil {
		t.Fatalf("read mysql schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply mysql schema: %v", err)
	}
}

func truncateMySQL(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM messages"); err != nil {
		t.Fatalf("truncate mysql messages: %v", err)
	}
}

func mysqlMessageCount(t *testing.T, db *sql.DB) int {
	t.Helper()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil {
		t.Fatalf("count mysql messages: %v", err)
	}
	return count
}

func waitForMySQLCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	waitFor(t, 20*time.Second, 200*time.Millisecond, fmt.Sprintf("mysql reaching %d rows", want), func() (bool, string) {
		got := mysqlMessageCount(t, db)
		return got == want, fmt.Sprintf("rows = %d", got)
	})
}

func mysqlMessages(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query("SELECT message FROM messages ORDER BY id")
	if err != nil {
		t.Fatalf("query mysql messages: %v", err)
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			t.Fatalf("scan mysql message: %v", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate mysql messages: %v", err)
	}
	return messages
}
