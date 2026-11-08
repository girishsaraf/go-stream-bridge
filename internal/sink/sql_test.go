package sink

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"gostreambridge/internal/config"
)

// TestSQLDriversRegistered guards against dropping the driver imports that
// register the MySQL and SQL Server implementations.
func TestSQLDriversRegistered(t *testing.T) {
	dsns := map[string]string{
		"mysql":     "user:pass@tcp(localhost:3306)/db",
		"sqlserver": "sqlserver://sa:pass@localhost:1433?database=db",
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
		Password: "p@ss/word",
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
	mysqlDSN, err := mysql.ParseDSN(mysqlSpec.dsn(cfg))
	if err != nil {
		t.Fatalf("parse mysql DSN: %v", err)
	}
	if mysqlDSN.User != "user" || mysqlDSN.Passwd != "p@ss/word" {
		t.Errorf("mysql credentials were not escaped: %+v", mysqlDSN)
	}
	if mysqlDSN.Addr != "localhost:1234" || mysqlDSN.DBName != "db" {
		t.Errorf("unexpected mysql DSN target: addr=%s db=%s", mysqlDSN.Addr, mysqlDSN.DBName)
	}
	if mysqlDSN.Timeout <= 0 || mysqlDSN.ReadTimeout <= 0 || mysqlDSN.WriteTimeout <= 0 {
		t.Errorf("mysql DSN should set connection timeouts: %+v", mysqlDSN)
	}

	sqlServerSpec, err := databaseSpecFor("sqlserver")
	if err != nil {
		t.Fatalf("databaseSpecFor(sqlserver): %v", err)
	}
	if !strings.Contains(sqlServerSpec.query, "@p1") || strings.Contains(sqlServerSpec.query, "?") {
		t.Errorf("sqlserver statement should use @pN placeholders: %s", sqlServerSpec.query)
	}
	parsed, err := url.Parse(sqlServerSpec.dsn(cfg))
	if err != nil {
		t.Fatalf("parse sqlserver DSN: %v", err)
	}
	if parsed.Scheme != "sqlserver" || parsed.Host != "localhost:1234" {
		t.Errorf("unexpected sqlserver DSN target: %s", parsed)
	}
	if parsed.User.Username() != "user" {
		t.Errorf("unexpected sqlserver user: %s", parsed.User)
	}
	password, _ := parsed.User.Password()
	if password != "p@ss/word" {
		t.Errorf("sqlserver password was not escaped: %q", password)
	}
	if parsed.Query().Get("database") != "db" {
		t.Errorf("unexpected sqlserver database: %s", parsed.Query().Get("database"))
	}
}
