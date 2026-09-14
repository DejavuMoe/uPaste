package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	filename     = "upaste.db"
	maxOpenConns = 4
)

func Open(ctx context.Context, dataDir string) (*sql.DB, error) {
	path, err := prepareDatabaseFile(dataDir)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return db, nil
}

func prepareDatabaseFile(dataDir string) (string, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	info, err := os.Stat(dataDir)
	if err != nil {
		return "", fmt.Errorf("inspect data directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("data path is not a directory: %s", dataDir)
	}

	path := filepath.Join(dataDir, filename)
	info, err = os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		file, createErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return "", fmt.Errorf("create database file: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", fmt.Errorf("close new database file: %w", closeErr)
		}
	case err != nil:
		return "", fmt.Errorf("inspect database file: %w", err)
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return "", fmt.Errorf("database path is not a regular file: %s", path)
	}
	return path, nil
}

func dsn(path string) string {
	query := url.Values{
		"_busy_timeout": {"5000"},
		"_defensive":    {"1"},
		"_dqs":          {"0"},
		"_foreign_keys": {"ON"},
		"_journal_mode": {"WAL"},
		"_synchronous":  {"NORMAL"},
		"_txlock":       {"immediate"},
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}).String()
}
