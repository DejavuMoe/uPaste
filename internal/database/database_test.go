package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenConfiguresEveryConnection(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if got := db.Stats().MaxOpenConnections; got != 4 {
		t.Fatalf("MaxOpenConnections = %d, want 4", got)
	}
	first, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	for i, conn := range []*sql.Conn{first, second} {
		t.Run(fmt.Sprintf("connection %d", i+1), func(t *testing.T) {
			assertPragmaText(t, conn, "journal_mode", "wal")
			assertPragmaInt(t, conn, "foreign_keys", 1)
			assertPragmaInt(t, conn, "busy_timeout", 5000)
			assertPragmaInt(t, conn, "synchronous", 1)

			if err := conn.QueryRowContext(ctx, `SELECT "missing_identifier"`).Scan(new(string)); err == nil {
				t.Fatal("DQS fallback is enabled")
			}

			var before, after int
			if err := conn.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&before); err != nil {
				t.Fatal(err)
			}
			if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA schema_version = %d", before+10)); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&after); err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatalf("defensive mode allowed schema_version change: %d -> %d", before, after)
			}
		})
	}

	var sqliteVersion string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&sqliteVersion); err != nil {
		t.Fatal(err)
	}
	if sqliteVersion != "3.53.4" {
		t.Fatalf("SQLite version = %q, want 3.53.4", sqliteVersion)
	}
}

func assertPragmaText(t *testing.T, conn *sql.Conn, name, want string) {
	t.Helper()
	var got string
	if err := conn.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, want) {
		t.Fatalf("PRAGMA %s = %q, want %q", name, got, want)
	}
}

func assertPragmaInt(t *testing.T, conn *sql.Conn, name string, want int) {
	t.Helper()
	var got int
	if err := conn.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("PRAGMA %s = %d, want %d", name, got, want)
	}
}

func TestOpenFilesystemPermissions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dataDir := filepath.Join(root, "new-data")
	db, err := Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	assertMode(t, dataDir, 0o700)
	assertMode(t, filepath.Join(dataDir, filename), 0o600)

	existingDir := filepath.Join(root, "existing-data")
	if err := os.Mkdir(existingDir, 0o750); err != nil {
		t.Fatal(err)
	}
	existingDB := filepath.Join(existingDir, filename)
	if err := os.WriteFile(existingDB, nil, 0o640); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, existingDir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	assertMode(t, existingDir, 0o750)
	assertMode(t, existingDB, 0o640)
}

func TestOpenRejectsInvalidFilesystemTypes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dataFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(dataFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, dataFile); err == nil {
		t.Fatal("data file unexpectedly accepted as a directory")
	}

	dataDir := filepath.Join(root, "database-is-directory")
	if err := os.Mkdir(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dataDir, filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, dataDir); err == nil {
		t.Fatal("directory unexpectedly accepted as database file")
	}

	symlinkDir := filepath.Join(root, "database-is-symlink")
	if err := os.Mkdir(symlinkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.db")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(symlinkDir, filename)); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, symlinkDir); err == nil {
		t.Fatal("symlink unexpectedly accepted as database file")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode for %s = %04o, want %04o", filepath.Base(path), got, want)
	}
}
