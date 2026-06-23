package database

import (
	"database/sql"
	"errors"
)

// ErrNoRows is an alias for sql.ErrNoRows so existing callers can keep using
// errors.Is(err, sql.ErrNoRows). Deprecated: use sql.ErrNoRows directly.
var ErrNoRows = sql.ErrNoRows

// Q rewrites a PostgreSQL-style query for the active backend. For SQLite it
// converts placeholders, dialect constructs, and type casts; for PostgreSQL it
// returns the query unchanged. Use it for every query executed against DB or a
// *sql.Tx when the backend is selected at runtime.
func Q(query string) string {
	if BackendType() == "sqlite" {
		return Rewrite(query)
	}
	return query
}

// Rewrite applies the SQLite dialect rewrite unconditionally. Most callers
// should use Q instead.
func Rewrite(query string) string {
	return rewrite(query)
}

// mapSQLError translates database/sql sentinel errors to database package
// errors. It is kept for compatibility with callers that relied on the previous
// adapter behaviour.
func mapSQLError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoRows
	}
	return err
}
