package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO required
)

// DB is the active database connection. It is set by Connect or ConnectSQLite.
var DB *sql.DB

// backend tracks which driver is in use.
var backend string

// PoolConfig contains database connection pool configuration.
type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultPoolConfig returns sensible defaults for connection pooling.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConns:          20,
		MinConns:          2,
		MaxConnLifetime:   1 * time.Hour,
		MaxConnIdleTime:   15 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}
}

// Connect establishes a PostgreSQL database connection using the pgx stdlib driver.
func Connect(ctx context.Context, connString string, poolConfig *PoolConfig) error {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if poolConfig == nil {
		poolConfig = DefaultPoolConfig()
	}

	db.SetMaxOpenConns(int(poolConfig.MaxConns))
	if poolConfig.MinConns > 0 {
		db.SetMaxIdleConns(int(poolConfig.MinConns))
	}
	db.SetConnMaxLifetime(poolConfig.MaxConnLifetime)
	db.SetConnMaxIdleTime(poolConfig.MaxConnIdleTime)
	// HealthCheckPeriod has no direct *sql.DB equivalent; pgx/stdlib handles
	// health checks internally via the connection pool.

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	backend = "postgres"
	return nil
}

// ConnectSQLite opens a SQLite database at dbPath and sets DB.
func ConnectSQLite(ctx context.Context, dbPath string) error {
	// WAL mode allows concurrent readers. Drop _txlock=immediate so reads don't
	// compete for the write lock. Multiple connections share the WAL safely.
	dsn := dbPath + "?_journal=WAL&_busy_timeout=30000&cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("connect sqlite: %w", err)
	}
	// Allow multiple concurrent readers; SQLite WAL handles the concurrency.
	// Keep writers serialized via busy_timeout rather than a single connection.
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("ping sqlite database: %w", err)
	}

	DB = db
	backend = "sqlite"
	return nil
}

// BackendType returns "sqlite" or "postgres" based on the active DB connection.
func BackendType() string {
	return backend
}

// Close closes the database connection pool.
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// Stats returns the current database pool statistics.
func Stats() sql.DBStats {
	if DB == nil {
		return sql.DBStats{}
	}
	return DB.Stats()
}

// HealthCheck pings the database and returns an error if it is unreachable.
func HealthCheck(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database connection not initialized")
	}
	return DB.PingContext(ctx)
}

// WithTx executes fn within a database transaction.
// If fn returns an error, the transaction is rolled back. Otherwise it is committed.
func WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original: %w)", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
