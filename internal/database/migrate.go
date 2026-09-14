package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

const latestSchemaVersion = 4

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version int
	path    string
}

var migrations = []migration{
	{1, "migrations/0001_shares.sql"},
	{2, "migrations/0002_standard_text_payloads.sql"},
	{3, "migrations/0003_encrypted_text_payloads.sql"},
	{4, "migrations/0004_file_payloads.sql"},
}

func migrate(ctx context.Context, db *sql.DB) error {
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > latestSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, latestSchemaVersion)
	}

	for _, migration := range migrations {
		if migration.version <= current {
			continue
		}
		if migration.version != current+1 {
			return fmt.Errorf("missing migration version %d", current+1)
		}
		sqlText, err := migrationFiles.ReadFile(migration.path)
		if err != nil {
			return fmt.Errorf("read migration %d: %w", migration.version, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.version, err)
		}
		if _, err = tx.ExecContext(ctx, string(sqlText)); err == nil {
			_, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", migration.version))
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}
		current = migration.version
	}
	if current != latestSchemaVersion {
		return fmt.Errorf("database schema version %d does not match supported version %d", current, latestSchemaVersion)
	}
	return nil
}
