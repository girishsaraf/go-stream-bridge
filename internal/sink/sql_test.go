package sink

import (
	"database/sql"
	"strings"
	"testing"

	"gostreambridge/internal/config"
)

// TestSQLDriversRegistered guards against dropping the blank imports that
// register the MySQL and SQL Server drivers.
func TestSQLDriversRegistered(t *testing.T) {
	dsns := map[string]string{
		"mysql":     "user:pass@tcp(localhost:3306)/db",
		"sqlserver": "server=localhost;user id=sa;password=pass;database=db",
	}

	for driver, dsn := range dsns {
		db, err := sql.Open(driver, dsn)
		if err != nil {
			t.Fatalf("driver %q is not registered: %v", driver, err)
		}
		if err := db.Close(); err != nil {
			t.Errorf("close %q database handle: %v", driver, err)
		}
	}
}

func TestDatabaseSpecs(t *testing.T) {
	cfg := config.Database{
		Username: "user",
		Password: "pass",
		Host:     "localhost",
		Port:     "1234",
		Database: "db",
	}

	mysqlSpec, err := databaseSpecFor("mysql")
	if err != nil {
		t.Fatalf("databaseSpecFor(mysql): %v", err)
	}
	if !strings.Contains(mysqlSpec.query, "?") {
		t.Errorf("mysql statement should use ? placeholders: %s", mysqlSpec.query)
	}
	if got := mysqlSpec.dsn(cfg); got != "user:pass@tcp(localhost:1234)/db" {
		t.Errorf("unexpected mysql DSN: %s", got)
	}

	sqlServerSpec, err := databaseSpecFor("sqlserver")
	if err != nil {
		t.Fatalf("databaseSpecFor(sqlserver): %v", err)
	}
	if !strings.Contains(sqlServerSpec.query, "@p1") || strings.Contains(sqlServerSpec.query, "?") {
		t.Errorf("sqlserver statement should use @pN placeholders: %s", sqlServerSpec.query)
	}
	if got := sqlServerSpec.dsn(cfg); !strings.Contains(got, "server=localhost") || !strings.Contains(got, "database=db") {
		t.Errorf("unexpected sqlserver DSN: %s", got)
	}
}
