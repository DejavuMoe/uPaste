package objectstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReconcileLocalObjects(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	staged, err := store.Stage(context.Background(), strings.NewReader("x"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Commit(); err != nil {
		t.Fatal(err)
	}
	path, _ := store.path(staged.Key())
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, now.Add(-31*time.Minute), now.Add(-31*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now.Add(-5*time.Minute), now.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err := store.Reconcile(context.Background(), map[string]struct{}{}, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.OrphansDeleted != 0 {
		t.Fatal("recent unreferenced object removed")
	}
	if err := os.Chtimes(path, now.Add(-31*time.Minute), now.Add(-31*time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err = store.Reconcile(context.Background(), map[string]struct{}{staged.Key(): {}}, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.OrphansDeleted != 0 {
		t.Fatal(result)
	}
	result, err = store.Reconcile(context.Background(), map[string]struct{}{}, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.OrphansDeleted != 1 {
		t.Fatal(result)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("stale orphan remains")
	}
}
func TestReconcileGraceStagesAndAnomalies(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	stage := filepath.Join(store.root, ".stage-old")
	if err := os.WriteFile(stage, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stage, now.Add(-31*time.Minute), now.Add(-31*time.Minute)); err != nil {
		t.Fatal(err)
	}
	recent := filepath.Join(store.root, ".stage-recent")
	if err := os.WriteFile(recent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recent, now.Add(-5*time.Minute), now.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(store.root, "keep-me")
	if err := os.WriteFile(unknown, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/tmp", filepath.Join(store.root, "ff")); err != nil {
		t.Fatal(err)
	}
	result, err := store.Reconcile(context.Background(), map[string]struct{}{strings.Repeat("a", 32): {}}, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.StagesDeleted != 1 || result.MissingReferenced != 1 || result.Anomalies < 2 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Lstat(recent); err != nil {
		t.Fatal("recent stage removed")
	}
	if _, err := os.Lstat(unknown); err != nil {
		t.Fatal("unknown removed")
	}
}
