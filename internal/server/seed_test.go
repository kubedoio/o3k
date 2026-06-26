package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// setupSeedTestDB opens an in-memory SQLite database and creates the minimal
// schema SeedDefaults touches.
func setupSeedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	if err := database.ConnectSQLite(ctx, ":memory:"); err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	t.Cleanup(database.Close)

	db := database.DB
	schemas := []string{
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY, name TEXT UNIQUE, description TEXT,
			enabled INTEGER, domain_id TEXT
		);`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY, name TEXT UNIQUE, password_hash TEXT,
			enabled INTEGER, domain_id TEXT
		);`,
		`CREATE TABLE roles (id TEXT PRIMARY KEY, name TEXT UNIQUE);`,
		`CREATE TABLE role_assignments (
			id TEXT PRIMARY KEY,
			user_id TEXT, project_id TEXT, role_id TEXT,
			UNIQUE(user_id, project_id, role_id)
		);`,
		`CREATE TABLE flavors (
			id TEXT PRIMARY KEY, name TEXT UNIQUE,
			vcpus INTEGER, ram_mb INTEGER, disk_gb INTEGER, is_public INTEGER
		);`,
		`CREATE TABLE flavor_extra_specs (
			flavor_id TEXT, key TEXT, value TEXT,
			PRIMARY KEY(flavor_id, key)
		);`,
		`CREATE TABLE volume_types (
			id TEXT PRIMARY KEY, name TEXT UNIQUE,
			description TEXT, is_public INTEGER,
			extra_specs TEXT DEFAULT '{}'
		);`,
	}
	for _, s := range schemas {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("create schema: %v\nSQL: %s", err, s)
		}
	}
	return db
}

// TestSeedDefaults_SCSVolumeTypes is the conformance check for Slice 5: after
// SeedDefaults runs, the three SCS-0114 reference volume types must exist with
// the expected extra-specs JSON document. This is the in-code mirror of
// migration 076 — keeps zero-config installs aligned with docker-compose.
func TestSeedDefaults_SCSVolumeTypes(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	if err := SeedDefaults(ctx, db, "test-password"); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	cases := []struct {
		name             string
		wantEncrypted    string
		wantReplicated   string
		wantAvailZone    string
		wantDescContains string
	}{
		{"scs-default", "false", "false", "nova", "single-AZ"},
		{"scs-encrypted", "true", "false", "nova", "[scs:encrypted]"},
		{"scs-replicated", "false", "true", "nova", "[scs:replicated]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var description, extraSpecsJSON string
			err := db.QueryRowContext(ctx,
				database.Q(`SELECT description, extra_specs FROM volume_types WHERE name = $1`),
				tc.name,
			).Scan(&description, &extraSpecsJSON)
			if err != nil {
				t.Fatalf("query volume_type %q: %v", tc.name, err)
			}

			if !contains(description, tc.wantDescContains) {
				t.Errorf("description = %q, want substring %q", description, tc.wantDescContains)
			}

			var extras map[string]string
			if err := json.Unmarshal([]byte(extraSpecsJSON), &extras); err != nil {
				t.Fatalf("parse extra_specs: %v (raw: %s)", err, extraSpecsJSON)
			}
			if got := extras["scs:encrypted"]; got != tc.wantEncrypted {
				t.Errorf("scs:encrypted = %q, want %q", got, tc.wantEncrypted)
			}
			if got := extras["scs:replicated"]; got != tc.wantReplicated {
				t.Errorf("scs:replicated = %q, want %q", got, tc.wantReplicated)
			}
			if got := extras["scs:availability-zone"]; got != tc.wantAvailZone {
				t.Errorf("scs:availability-zone = %q, want %q", got, tc.wantAvailZone)
			}
		})
	}
}

// TestSeedDefaults_Idempotent: SeedDefaults short-circuits when the admin user
// already exists, so a second call must NOT duplicate or alter the volume
// types. The first call seeds, the second is a no-op — both calls return nil.
func TestSeedDefaults_Idempotent(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	if err := SeedDefaults(ctx, db, "test-password"); err != nil {
		t.Fatalf("first SeedDefaults: %v", err)
	}
	if err := SeedDefaults(ctx, db, "test-password"); err != nil {
		t.Fatalf("second SeedDefaults: %v", err)
	}

	var count int
	err := db.QueryRowContext(ctx,
		database.Q(`SELECT COUNT(*) FROM volume_types WHERE name LIKE 'scs-%'`),
	).Scan(&count)
	if err != nil {
		t.Fatalf("count scs volume_types: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 scs-* volume types after double-seed, got %d", count)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestSeedDefaults_PasswordUpdate: when admin already exists and a new password
// is supplied, SeedDefaults must update the stored hash.
func TestSeedDefaults_PasswordUpdate(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	if err := SeedDefaults(ctx, db, "first-password"); err != nil {
		t.Fatalf("initial seed: %v", err)
	}

	// Re-seed with a different password — must update the hash.
	if err := SeedDefaults(ctx, db, "second-password"); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	var hash string
	if err := db.QueryRowContext(ctx,
		database.Q("SELECT password_hash FROM users WHERE name = $1"), "admin",
	).Scan(&hash); err != nil {
		t.Fatalf("read hash: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("second-password")); err != nil {
		t.Errorf("stored hash does not match second-password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("first-password")); err == nil {
		t.Errorf("stored hash still matches first-password after update")
	}
}
