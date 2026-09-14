package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateFreshAndReopen(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	db, err := Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	assertSchemaVersion(t, db, latestSchemaVersion)
	assertSchemaObject(t, db, "table", "shares")
	assertSchemaObject(t, db, "index", "shares_expires_at_idx")
	var strict, partial int
	if err := db.QueryRow("SELECT strict FROM pragma_table_list WHERE name='shares'").Scan(&strict); err != nil {
		t.Fatal(err)
	}
	if strict != 1 {
		t.Fatal("shares table is not STRICT")
	}
	if err := db.QueryRow("SELECT partial FROM pragma_index_list('shares') WHERE name='shares_expires_at_idx'").Scan(&partial); err != nil {
		t.Fatal(err)
	}
	if partial != 1 {
		t.Fatal("expiration index is not partial")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, dataDir)
	if err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
	defer db.Close()
	assertSchemaVersion(t, db, latestSchemaVersion)
}

func TestMigrateRejectsNewerSchema(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, filename)
	db := openRaw(t, path)
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(context.Background(), dataDir); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("Open error = %v, want newer-schema refusal", err)
	}
}

func TestFailedMigrationDoesNotAdvanceVersion(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, filename)
	db := openRaw(t, path)
	if _, err := db.Exec(`
		CREATE TABLE blocker (expires_at INTEGER);
		CREATE INDEX shares_expires_at_idx ON blocker(expires_at);
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(context.Background(), dataDir); err == nil {
		t.Fatal("migration unexpectedly succeeded")
	}
	db = openRaw(t, path)
	defer db.Close()
	assertSchemaVersion(t, db, 0)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='shares'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration left the shares table behind")
	}
}

func TestSharesSchemaConstraints(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	validID := "AAAAAAAAAAAAAAAAAAAAAA"
	validVerifier := make([]byte, 32)
	tests := []struct {
		name      string
		id        string
		kind      string
		privacy   string
		verifier  any
		expiresAt any
	}{
		{"invalid ID length", "short", "TEXT", "STANDARD", validVerifier, nil},
		{"invalid payload kind", validID, "OTHER", "STANDARD", validVerifier, nil},
		{"invalid privacy mode", validID, "TEXT", "OTHER", validVerifier, nil},
		{"encrypted file", validID, "FILE", "ENCRYPTED", validVerifier, nil},
		{"short verifier", validID, "TEXT", "STANDARD", make([]byte, 31), nil},
		{"long verifier", validID, "TEXT", "STANDARD", make([]byte, 33), nil},
		{"text verifier", validID, "TEXT", "STANDARD", strings.Repeat("x", 32), nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := insertMetadata(db, test.id, test.kind, test.privacy, test.verifier, test.expiresAt); err == nil {
				t.Fatal("invalid metadata unexpectedly inserted")
			}
		})
	}

	if err := insertMetadata(db, validID, "TEXT", "ENCRYPTED", validVerifier, nil); err != nil {
		t.Fatalf("insert valid metadata with nullable expires_at: %v", err)
	}
}

func insertMetadata(db *sql.DB, id, kind, privacy string, verifier, expiresAt any) error {
	_, err := db.Exec(`
		INSERT INTO shares (
			id, payload_kind, privacy_mode, owner_token_verifier,
			created_at, updated_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, kind, privacy, verifier, int64(1), int64(1), expiresAt)
	return err
}

func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}

func assertSchemaVersion(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("PRAGMA user_version").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
}

func assertSchemaObject(t *testing.T, db *sql.DB, objectType, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE type=? AND name=?", objectType, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("%s %q count = %d, want 1", objectType, name, count)
	}
}
