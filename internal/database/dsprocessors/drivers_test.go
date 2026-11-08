package dsprocessors

import (
	"database/sql"
	"strings"
	"testing"
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

func TestInsertStatementPlaceholders(t *testing.T) {
	mysqlQuery, err := insertStatement("mysql")
	if err != nil {
		t.Fatalf("insertStatement(mysql): %v", err)
	}
	if !strings.Contains(mysqlQuery, "?") {
		t.Errorf("mysql statement should use ? placeholders: %s", mysqlQuery)
	}

	sqlServerQuery, err := insertStatement("sqlserver")
	if err != nil {
		t.Fatalf("insertStatement(sqlserver): %v", err)
	}
	if !strings.Contains(sqlServerQuery, "@p1") || strings.Contains(sqlServerQuery, "?") {
		t.Errorf("sqlserver statement should use @pN placeholders: %s", sqlServerQuery)
	}
}
