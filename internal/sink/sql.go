package sink

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/go-sql-driver/mysql"
	mssql "github.com/microsoft/go-mssqldb"

	"gostreambridge/internal/config"
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

	pingErr := db.PingContext(setupCtx)
	if pingErr != nil {
		return nil, errors.Join(
			fmt.Errorf("ping %s database: %w", driver, pingErr),
			db.Close(),
		)
	}

	stmt, err := db.PrepareContext(setupCtx, spec.query)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("prepare %s statement: %w", driver, err),
			db.Close(),
		)
	}

	return &SQLSink{db: db, stmt: stmt, driver: driver}, nil
}

// Write inserts one message. Retrying is handled centrally by the pipeline;
// this method performs exactly one attempt and classifies permanent failures.
func (s *SQLSink) Write(ctx context.Context, message []byte) error {
	_, err := s.stmt.ExecContext(ctx, string(message), time.Now())
	return classifySQLError(s.driver, err)
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
				mysqlCfg := mysql.NewConfig()
				mysqlCfg.User = cfg.Username
				mysqlCfg.Passwd = cfg.Password
				mysqlCfg.Net = "tcp"
				mysqlCfg.Addr = net.JoinHostPort(cfg.Host, cfg.Port)
				mysqlCfg.DBName = cfg.Database
				mysqlCfg.ParseTime = true
				mysqlCfg.Timeout = 5 * time.Second
				mysqlCfg.ReadTimeout = 30 * time.Second
				mysqlCfg.WriteTimeout = 30 * time.Second
				return mysqlCfg.FormatDSN()
			},
		}, nil
	case "sqlserver":
		return databaseSpec{
			configFile: "sqlserver.json",
			// The sqlserver driver uses native @pN placeholders; it does not
			// translate "?".
			query: "INSERT INTO messages (message, timestamp) VALUES (@p1, @p2)",
			dsn: func(cfg config.Database) string {
				query := url.Values{}
				query.Set("database", cfg.Database)
				return (&url.URL{
					Scheme:   "sqlserver",
					User:     url.UserPassword(cfg.Username, cfg.Password),
					Host:     net.JoinHostPort(cfg.Host, cfg.Port),
					RawQuery: query.Encode(),
				}).String()
			},
		}, nil
	default:
		return databaseSpec{}, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// classifySQLError marks schema, data, and authorization failures as permanent.
// Deadlocks, lock timeouts, and connection errors remain retryable.
func classifySQLError(driver string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	switch driver {
	case "mysql":
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && permanentMySQLCodes[mysqlErr.Number] {
			return Permanent(err)
		}
	case "sqlserver":
		var sqlErr mssql.Error
		if errors.As(err, &sqlErr) && permanentSQLServerCodes[sqlErr.Number] {
			return Permanent(err)
		}
	}
	return err
}

var permanentMySQLCodes = map[uint16]bool{
	1022: true, // duplicate key
	1048: true, // column cannot be null
	1054: true, // unknown column
	1062: true, // duplicate entry
	1064: true, // syntax error
	1146: true, // table does not exist
	1264: true, // out of range
	1366: true, // incorrect string value
	1406: true, // data too long
	1452: true, // foreign key constraint
	3819: true, // check constraint violation
}

var permanentSQLServerCodes = map[int32]bool{
	102:  true, // syntax error
	207:  true, // invalid column
	208:  true, // invalid object
	229:  true, // permission denied
	245:  true, // conversion failed
	515:  true, // cannot insert null
	547:  true, // constraint violation
	2601: true, // duplicate key
	2627: true, // unique constraint
	2628: true, // string truncation
	8114: true, // conversion error
	8152: true, // string truncation
}
