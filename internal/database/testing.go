package database

import (
	"context"
	"database/sql"
	"testing"

	migrations "github.com/cobaltcore-dev/o3k/migrations"
)

// NewTestDB opens an in-memory SQLite database, runs the embedded SQLite
// migrations, and returns the *sql.DB. The database is closed automatically
// when the test ends.
func NewTestDB(t testing.TB) *sql.DB {
	t.Helper()
	ctx := context.Background()
	if err := ConnectSQLite(ctx, ":memory:"); err != nil {
		t.Fatalf("connect test sqlite: %v", err)
	}
	t.Cleanup(Close)
	if err := MigrateSQLiteFS(migrations.SQLiteFS); err != nil {
		t.Fatalf("migrate test sqlite: %v", err)
	}
	return DB
}
