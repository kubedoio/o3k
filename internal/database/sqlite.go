package database

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// placeholderRegex matches PostgreSQL positional parameters ($1, $2, ...).
var placeholderRegex = regexp.MustCompile(`\$\d+`)

// rewritePlaceholders converts PostgreSQL $N parameters to SQLite ?N parameters.
// Using the ?NNN form preserves PostgreSQL's duplicate-reference semantics
// (e.g. "$1 OR $1" becomes "?1 OR ?1" and still binds a single argument).
func rewritePlaceholders(query string) string {
	return placeholderRegex.ReplaceAllStringFunc(query, func(match string) string {
		return "?" + match[1:]
	})
}

// castRegex matches PostgreSQL type-cast suffixes (::text, ::jsonb, ::varchar(N), etc.).
var castRegex = regexp.MustCompile(`(?i)::\s*(?:double\s+precision|timestamp(?:\s+(?:with|without)\s+time\s+zone)?|varchar(?:\(\d+\))?|numeric(?:\(\d+(?:,\s*\d+)?\))?|char(?:\(\d+\))?|smallint|bigint|integer|int|jsonb|json|boolean|uuid|real|float|interval|date|time|bytea|inet|cidr|text)`)

// rewriteDialect rewrites PostgreSQL-specific syntax to SQLite equivalents.
func rewriteDialect(query string) string {
	// ILIKE → LIKE (SQLite LIKE is case-insensitive for ASCII by default)
	query = strings.ReplaceAll(query, " ILIKE ", " LIKE ")
	query = strings.ReplaceAll(query, " ilike ", " LIKE ")

	// NOW() → CURRENT_TIMESTAMP
	query = strings.ReplaceAll(query, "NOW()", "CURRENT_TIMESTAMP")
	query = strings.ReplaceAll(query, "now()", "CURRENT_TIMESTAMP")

	// Remove PostgreSQL type casts: ::text, ::int, ::bigint, etc.
	query = castRegex.ReplaceAllString(query, "")

	// EXTRACT(EPOCH FROM (...)) → CAST(strftime('%s', ...) AS INTEGER)
	query = rewriteExtractEpoch(query)

	// Remove row-locking clauses SQLite handles via BEGIN IMMEDIATE
	query = strings.ReplaceAll(query, " FOR UPDATE SKIP LOCKED", "")
	query = strings.ReplaceAll(query, " FOR UPDATE", "")

	return query
}

// rewriteExtractEpoch rewrites all EXTRACT(EPOCH FROM (...)) occurrences in
// query to CAST(strftime('%s', ...) AS INTEGER). It uses a balanced-paren
// scanner so nested function calls inside the expression are handled correctly.
func rewriteExtractEpoch(query string) string {
	const prefix = "EXTRACT(EPOCH FROM ("
	var b strings.Builder
	for {
		idx := strings.Index(query, prefix)
		if idx == -1 {
			b.WriteString(query)
			break
		}
		b.WriteString(query[:idx])

		rest := query[idx+len(prefix):]
		depth := 1
		closeIdx := -1
		for i, ch := range rest {
			switch ch {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					closeIdx = i
				}
			}
			if closeIdx != -1 {
				break
			}
		}
		if closeIdx == -1 {
			b.WriteString(query[idx:])
			break
		}
		inner := rest[:closeIdx]
		if parts := strings.SplitN(inner, " - ", 2); len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			fmt.Fprintf(&b, "(CAST(strftime('%%s', %s) AS INTEGER) - CAST(strftime('%%s', %s) AS INTEGER))", left, right)
		} else {
			fmt.Fprintf(&b, "CAST(strftime('%%s', %s) AS INTEGER)", inner)
		}

		after := rest[closeIdx+1:]
		if len(after) > 0 && after[0] == ')' {
			after = after[1:]
		}
		query = after
	}
	return b.String()
}

// rewrite applies placeholder and dialect rewrites in one step.
func rewrite(query string) string {
	return rewriteDialect(rewritePlaceholders(query))
}

// migrationVersionRegex extracts the numeric prefix from migration filenames like "001_initial_schema.up.sql".
var migrationVersionRegex = regexp.MustCompile(`^(\d+)_.+\.up\.sql$`)

// MigrateSQLiteFS applies SQLite migration files from an fs.FS (typically an
// embed.FS) under the "sqlite" directory, in sorted order. It tracks applied
// migrations in a schema_migrations table and is idempotent.
func MigrateSQLiteFS(fsys fs.FS) error {
	if DB == nil || BackendType() != "sqlite" {
		return fmt.Errorf("MigrateSQLiteFS called but DB is not SQLite")
	}

	// Create schema_migrations tracking table if not exists.
	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(fsys, "sqlite")
	if err != nil {
		return fmt.Errorf("read sqlite migrations from embedded FS: %w", err)
	}

	type migration struct {
		version  string
		filename string
	}
	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationVersionRegex.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		migrations = append(migrations, migration{version: matches[1], filename: entry.Name()})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	applied := 0
	for _, m := range migrations {
		var exists string
		err := DB.QueryRow("SELECT version FROM schema_migrations WHERE version = ?", m.version).Scan(&exists)
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", m.version, err)
		}

		content, err := fs.ReadFile(fsys, filepath.Join("sqlite", m.filename))
		if err != nil {
			return fmt.Errorf("read embedded migration file %s: %w", m.filename, err)
		}

		tx, err := DB.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction for migration %s: %w", m.version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", m.version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.version, err)
		}
		applied++
	}

	if applied == 0 {
		fmt.Println("SQLite database is already up to date")
	} else {
		fmt.Printf("Applied %d SQLite migration(s)\n", applied)
	}
	return nil
}

// MigrateSQLite applies SQLite migration files from {migrationsPath}/sqlite/ in sorted order.
func MigrateSQLite(migrationsPath string) error {
	if DB == nil || BackendType() != "sqlite" {
		return fmt.Errorf("MigrateSQLite called but DB is not SQLite")
	}

	sqliteDir := filepath.Join(migrationsPath, "sqlite")
	if _, err := os.Stat(sqliteDir); os.IsNotExist(err) {
		return fmt.Errorf("sqlite migrations directory does not exist: %s", sqliteDir)
	}

	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(sqliteDir)
	if err != nil {
		return fmt.Errorf("read sqlite migrations directory: %w", err)
	}

	type migration struct {
		version  string
		filename string
	}
	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationVersionRegex.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		migrations = append(migrations, migration{version: matches[1], filename: entry.Name()})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	applied := 0
	for _, m := range migrations {
		var exists string
		err := DB.QueryRow("SELECT version FROM schema_migrations WHERE version = ?", m.version).Scan(&exists)
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", m.version, err)
		}

		content, err := os.ReadFile(filepath.Join(sqliteDir, m.filename))
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", m.filename, err)
		}

		tx, err := DB.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction for migration %s: %w", m.version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", m.version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.version, err)
		}
		applied++
	}

	if applied == 0 {
		fmt.Println("SQLite database is already up to date")
	} else {
		fmt.Printf("Applied %d SQLite migration(s)\n", applied)
	}
	return nil
}
