package sink

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
	"gostreambridge/internal/retry"
)

const (
	sqlStartupTimeout  = 15 * time.Second
	sqlMaxOpenConns    = 10
	sqlMaxIdleConns    = 5
	sqlConnMaxLifetime = 30 * time.Minute
)

// SQLSink writes messages to MySQL or SQL Server using a long-lived connection
// pool and prepared statement.
type SQLSink struct {
	db     *sql.DB
	stmt   *sql.Stmt
	driver string
}

// NewSQLSink loads the database configuration and opens a connection pool.
func NewSQLSink(ctx context.Context, driver, configDir string) (*SQLSink, error) {
	spec, err := databaseSpecFor(driver)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load[config.Database](config.File(configDir, spec.configFile))
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driver, spec.dsn(cfg))
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", driver, err)
	}

	db.SetMaxOpenConns(sqlMaxOpenConns)
	db.SetMaxIdleConns(sqlMaxIdleConns)
	db.SetConnMaxLifetime(sqlConnMaxLifetime)

	setupCtx, cancel := context.WithTimeout(ctx, sqlStartupTimeout)
	defer cancel()

	if err := db.PingContext(setupCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s database: %w", driver, err)
	}

	stmt, err := db.PrepareContext(setupCtx, spec.query)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare %s statement: %w", driver, err)
	}

	return &SQLSink{db: db, stmt: stmt, driver: driver}, nil
}

// Write inserts one message, retrying transient failures with backoff.
func (s *SQLSink) Write(ctx context.Context, message []byte) error {
	policy := retry.Default()
	policy.OnRetry = func(attempt int, err error) {
		log.Warn().
			Err(err).
			Str("driver", s.driver).
			Int("attempt", attempt).
			Msg("failed to insert message")
	}

	return policy.Do(ctx, func(ctx context.Context) error {
		_, err := s.stmt.ExecContext(ctx, string(message), time.Now())
		return err
	})
}

// Close releases the prepared statement and connection pool.
func (s *SQLSink) Close() error {
	return errors.Join(s.stmt.Close(), s.db.Close())
}

// databaseSpec centralizes the driver-specific configuration needed by the
// SQL sink.
type databaseSpec struct {
	configFile string
	query      string
	dsn        func(config.Database) string
}

func databaseSpecFor(driver string) (databaseSpec, error) {
	switch driver {
	case "mysql":
		return databaseSpec{
			configFile: "mysql.json",
			query:      "INSERT INTO messages (message, timestamp) VALUES (?, ?)",
			dsn: func(cfg config.Database) string {
				return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
			},
		}, nil
	case "sqlserver":
		return databaseSpec{
			configFile: "sqlserver.json",
			// The sqlserver driver uses native @pN placeholders; it does not
			// translate "?".
			query: "INSERT INTO messages (message, timestamp) VALUES (@p1, @p2)",
			dsn: func(cfg config.Database) string {
				return fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s", cfg.Host, cfg.Username, cfg.Password, cfg.Port, cfg.Database)
			},
		}, nil
	default:
		return databaseSpec{}, fmt.Errorf("unsupported database driver: %s", driver)
	}
}
