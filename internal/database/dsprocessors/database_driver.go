package dsprocessors

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"gostreambridge/pkg/util"
)

const (
	dbMaxRetries  = 5
	dbRetryDelay  = 2 * time.Second
	dbPingTimeout = 10 * time.Second
)

// WriteToDatabase writes a message to the configured MySQL or SQL Server
// database. The message is committed to the database before returning nil.
func WriteToDatabase(ctx context.Context, driver, configFile, message string) error {
	dbConfig, err := util.ConvertConfigFileToMap(configFile)
	if err != nil {
		return fmt.Errorf("load %s config file: %w", driver, err)
	}

	if dbConfig["host"] == "" || dbConfig["database"] == "" {
		return fmt.Errorf("%s config: host and database must not be empty", driver)
	}

	var dsn string
	switch driver {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbConfig["username"], dbConfig["password"], dbConfig["host"], dbConfig["port"], dbConfig["database"])
	case "sqlserver":
		dsn = fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s", dbConfig["host"], dbConfig["username"], dbConfig["password"], dbConfig["port"], dbConfig["database"])
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}

	query, err := insertStatement(driver)
	if err != nil {
		return err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("open %s database: %w", driver, err)
	}
	defer db.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, dbPingTimeout)
	defer cancelPing()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping %s database: %w", driver, err)
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare %s statement: %w", driver, err)
	}
	defer stmt.Close()

	var lastErr error
	for attempt := 1; attempt <= dbMaxRetries; attempt++ {
		_, execErr := stmt.ExecContext(ctx, message, time.Now())
		if execErr == nil {
			return nil
		}
		lastErr = execErr

		if attempt == dbMaxRetries {
			break
		}

		log.Warn().
			Err(lastErr).
			Str("driver", driver).
			Int("attempt", attempt).
			Int("maxAttempts", dbMaxRetries).
			Msg("failed to execute insert")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(dbRetryDelay):
		}
	}

	return fmt.Errorf("insert into %s after %d attempts: %w", driver, dbMaxRetries, lastErr)
}

// insertStatement returns the driver-specific INSERT statement. The
// "sqlserver" driver uses native @pN placeholders; it does not translate "?".
func insertStatement(driver string) (string, error) {
	switch driver {
	case "mysql":
		return "INSERT INTO messages (message, timestamp) VALUES (?, ?)", nil
	case "sqlserver":
		return "INSERT INTO messages (message, timestamp) VALUES (@p1, @p2)", nil
	default:
		return "", fmt.Errorf("unsupported database driver: %s", driver)
	}
}
