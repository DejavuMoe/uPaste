package objectstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalRootSurvivesPathReplacement(t *testing.T) {
	data := t.TempDir()
	store, err := OpenLocal(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	moved := filepath.Join(data, "moved")
	if err := os.Rename(store.root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(store.root, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(store.root, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside root handle"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged, err := store.Stage(context.Background(), strings.NewReader("x"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.confined.Lstat(staged.Key()[:2] + "/" + staged.Key()[2:]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.root, staged.Key()[:2], staged.Key()[2:])); !os.IsNotExist(err) {
		t.Fatal("replacement pathname received object")
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "outside root handle" {
		t.Fatal("replacement sentinel changed")
	}
}

func TestLocalRootRejectsExternalSymlinks(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	key := "aa" + strings.Repeat("b", 30)
	if err := os.Symlink(outside, filepath.Join(store.root, "aa")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(key); err == nil {
		t.Fatal("shard symlink deleted")
	}
	result, err := store.Reconcile(context.Background(), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.Anomalies == 0 {
		t.Fatal("shard symlink not reported")
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "keep" {
		t.Fatal("outside sentinel changed")
	}

	if err := os.Remove(filepath.Join(store.root, "aa")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(store.root, "aa"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(store.root, "aa", strings.Repeat("b", 30))); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(key); err == nil {
		t.Fatal("final symlink deleted")
	}
	result, err = store.Reconcile(context.Background(), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.Anomalies == 0 {
		t.Fatal("final symlink not reported")
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "keep" {
		t.Fatal("final sentinel changed")
	}
}

func TestLocalCloseIsIdempotent(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stage(context.Background(), strings.NewReader("x"), 1); err == nil {
		t.Fatal("closed store staged")
	}
}
