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
	assertSchemaObject(t, db, "table", "standard_text_payloads")
	assertSchemaObject(t, db, "table", "encrypted_text_payloads")
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
	if _, err := db.Exec("PRAGMA user_version = 4"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(context.Background(), dataDir); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("Open error = %v, want newer-schema refusal", err)
	}
}

func TestMigrateVersionOneToThree(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, filename)
	db := openRaw(t, path)
	migrationOne, err := migrationFiles.ReadFile("migrations/0001_shares.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migrationOne)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	id := "AAAAAAAAAAAAAAAAAAAAAA"
	verifier := make([]byte, 32)
	if err := insertMetadata(db, id, "TEXT", "STANDARD", verifier, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assertSchemaVersion(t, db, 3)
	assertSchemaObject(t, db, "table", "standard_text_payloads")
	assertSchemaObject(t, db, "table", "encrypted_text_payloads")
	var kind, privacy string
	var storedVerifier []byte
	var createdAt, updatedAt int64
	var expiresAt any
	if err := db.QueryRow(`
		SELECT payload_kind, privacy_mode, owner_token_verifier, created_at, updated_at, expires_at
		FROM shares WHERE id = ?
	`, id).Scan(&kind, &privacy, &storedVerifier, &createdAt, &updatedAt, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if kind != "TEXT" || privacy != "STANDARD" || string(storedVerifier) != string(verifier) || createdAt != 1 || updatedAt != 1 || expiresAt != nil {
		t.Fatal("version 1 metadata changed during migration")
	}
}

func TestMigrateVersionTwoToThreePreservesStandardText(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, filename)
	db := openRaw(t, path)
	for _, name := range []string{"migrations/0001_shares.sql", "migrations/0002_standard_text_payloads.sql"} {
		sqlText, err := migrationFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(sqlText)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	id := "BBBBBBBBBBBBBBBBBBBBBB"
	if err := insertMetadata(db, id, "TEXT", "STANDARD", make([]byte, 32), nil); err != nil {
		t.Fatal(err)
	}
	content := "existing \x00 Standard text ☃"
	if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, 'MARKDOWN', ?)", id, content); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assertSchemaVersion(t, db, 3)
	var format, stored string
	if err := db.QueryRow("SELECT format, content FROM standard_text_payloads WHERE share_id = ?", id).Scan(&format, &stored); err != nil {
		t.Fatal(err)
	}
	if format != "MARKDOWN" || stored != content {
		t.Fatal("version 2 Standard Text changed during migration")
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

func TestFailedEncryptedMigrationDoesNotAdvanceVersion(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, filename)
	db := openRaw(t, path)
	for _, name := range []string{"migrations/0001_shares.sql", "migrations/0002_standard_text_payloads.sql"} {
		sqlText, err := migrationFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(sqlText)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 2; CREATE TABLE encrypted_text_payloads (share_id TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), dataDir); err == nil {
		t.Fatal("migration 3 unexpectedly succeeded")
	}
	db = openRaw(t, path)
	defer db.Close()
	assertSchemaVersion(t, db, 2)
	var triggerCount int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE type = 'trigger'").Scan(&triggerCount); err != nil {
		t.Fatal(err)
	}
	if triggerCount != 0 {
		t.Fatal("failed migration 3 left triggers behind")
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

func TestStandardTextPayloadSchema(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var strict int
	if err := db.QueryRow("SELECT strict FROM pragma_table_list WHERE name='standard_text_payloads'").Scan(&strict); err != nil {
		t.Fatal(err)
	}
	if strict != 1 {
		t.Fatal("standard_text_payloads is not STRICT")
	}

	verifier := make([]byte, 32)
	formats := []string{"PLAIN", "SOURCE", "MARKDOWN"}
	for i, format := range formats {
		id := strings.Repeat(string(rune('A'+i)), 22)
		if err := insertMetadata(db, id, "TEXT", "STANDARD", verifier, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, ?, ?)", id, format, "x"); err != nil {
			t.Errorf("insert format %s: %v", format, err)
		}
	}

	constraintID := strings.Repeat("D", 22)
	if err := insertMetadata(db, constraintID, "TEXT", "STANDARD", verifier, nil); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, format, content string
	}{
		{"invalid format", "OTHER", "x"},
		{"empty content", "PLAIN", ""},
		{"oversized content", "PLAIN", strings.Repeat("x", 1048577)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, ?, ?)", constraintID, test.format, test.content); err == nil {
				t.Fatal("invalid payload unexpectedly inserted")
			}
		})
	}

	maxID := strings.Repeat("E", 22)
	if err := insertMetadata(db, maxID, "TEXT", "STANDARD", verifier, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, 'PLAIN', ?)", maxID, strings.Repeat("x", 1048576)); err != nil {
		t.Fatalf("insert exactly 1 MiB: %v", err)
	}

	if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, 'PLAIN', 'x')", strings.Repeat("F", 22)); err == nil {
		t.Fatal("payload without Share unexpectedly inserted")
	}
	if _, err := db.Exec("DELETE FROM shares WHERE id = ?", maxID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM standard_text_payloads WHERE share_id = ?", maxID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("payload did not cascade on Share deletion")
	}
}

func TestEncryptedTextPayloadSchema(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var strict int
	if err := db.QueryRow("SELECT strict FROM pragma_table_list WHERE name='encrypted_text_payloads'").Scan(&strict); err != nil {
		t.Fatal(err)
	}
	if strict != 1 {
		t.Fatal("encrypted_text_payloads is not STRICT")
	}

	insertParent := func(id, privacy string) {
		t.Helper()
		if err := insertMetadata(db, id, "TEXT", privacy, make([]byte, 32), nil); err != nil {
			t.Fatal(err)
		}
	}
	insertEncrypted := func(id string, protocol any, nonce any, ciphertext any) error {
		_, err := db.Exec("INSERT INTO encrypted_text_payloads (share_id, protocol, nonce, ciphertext) VALUES (?, ?, ?, ?)", id, protocol, nonce, ciphertext)
		return err
	}

	validID := strings.Repeat("G", 22)
	insertParent(validID, "ENCRYPTED")
	if err := insertEncrypted(validID, "UPASTE_AES_GCM_V1", make([]byte, 12), make([]byte, 19)); err != nil {
		t.Fatalf("insert minimum encrypted payload: %v", err)
	}

	for i, test := range []struct {
		name       string
		protocol   any
		nonce      any
		ciphertext any
	}{
		{"unsupported protocol", "OTHER", make([]byte, 12), make([]byte, 19)},
		{"nonce text", "UPASTE_AES_GCM_V1", strings.Repeat("n", 12), make([]byte, 19)},
		{"short nonce", "UPASTE_AES_GCM_V1", make([]byte, 11), make([]byte, 19)},
		{"ciphertext text", "UPASTE_AES_GCM_V1", make([]byte, 12), strings.Repeat("c", 19)},
		{"short ciphertext", "UPASTE_AES_GCM_V1", make([]byte, 12), make([]byte, 18)},
		{"oversized ciphertext", "UPASTE_AES_GCM_V1", make([]byte, 12), make([]byte, 1048595)},
	} {
		t.Run(test.name, func(t *testing.T) {
			id := strings.Repeat(string(rune('H'+i)), 22)
			insertParent(id, "ENCRYPTED")
			if err := insertEncrypted(id, test.protocol, test.nonce, test.ciphertext); err == nil {
				t.Fatal("invalid encrypted payload unexpectedly inserted")
			}
		})
	}

	maxID := strings.Repeat("N", 22)
	insertParent(maxID, "ENCRYPTED")
	if err := insertEncrypted(maxID, "UPASTE_AES_GCM_V1", make([]byte, 12), make([]byte, 1048594)); err != nil {
		t.Fatalf("insert maximum encrypted payload: %v", err)
	}

	wrongEncryptedID := strings.Repeat("O", 22)
	insertParent(wrongEncryptedID, "STANDARD")
	if err := insertEncrypted(wrongEncryptedID, "UPASTE_AES_GCM_V1", make([]byte, 12), make([]byte, 19)); err == nil {
		t.Fatal("encrypted payload accepted a Standard parent")
	}
	if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, 'PLAIN', 'x')", wrongEncryptedID); err != nil {
		t.Fatal(err)
	}
	wrongStandardID := strings.Repeat("P", 22)
	insertParent(wrongStandardID, "ENCRYPTED")
	if _, err := db.Exec("INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, 'PLAIN', 'x')", wrongStandardID); err == nil {
		t.Fatal("Standard payload accepted an Encrypted parent")
	}
	if _, err := db.Exec("UPDATE encrypted_text_payloads SET share_id = ? WHERE share_id = ?", wrongEncryptedID, validID); err == nil {
		t.Fatal("encrypted payload moved to a Standard parent")
	}
	if _, err := db.Exec("UPDATE standard_text_payloads SET share_id = ? WHERE share_id = ?", wrongStandardID, wrongEncryptedID); err == nil {
		t.Fatal("Standard payload moved to an Encrypted parent")
	}
	if _, err := db.Exec("UPDATE shares SET privacy_mode = 'STANDARD' WHERE id = ?", maxID); err == nil {
		t.Fatal("encrypted parent privacy changed beneath its payload")
	}
	if _, err := db.Exec("UPDATE shares SET privacy_mode = 'ENCRYPTED' WHERE id = ?", wrongEncryptedID); err == nil {
		t.Fatal("Standard parent privacy changed beneath its payload")
	}

	if _, err := db.Exec("DELETE FROM shares WHERE id = ?", maxID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM encrypted_text_payloads WHERE share_id = ?", maxID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("encrypted payload did not cascade")
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
