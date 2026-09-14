package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStageCommitOpenDelete(t *testing.T) {
	dataDir := t.TempDir()
	store, err := OpenLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := store.Stage(context.Background(), strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.Key()) != 32 || staged.Size() != 5 || staged.SHA256() != sha256.Sum256([]byte("hello")) || string(staged.Sniff()) != "hello" {
		t.Fatal("staged metadata is incorrect")
	}
	if err := staged.Commit(); err != nil {
		t.Fatal(err)
	}
	object, err := store.Open(staged.Key())
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(object)
	if closeErr := object.Close(); err == nil {
		err = closeErr
	}
	if err != nil || string(content) != "hello" {
		t.Fatalf("open content/error = %q/%v", content, err)
	}
	path, err := store.path(staged.Key())
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Join(dataDir, "objects"), 0o700)
	assertMode(t, path, 0o600)
	if err := store.Delete(staged.Key()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(staged.Key()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open deleted object error = %v", err)
	}
	if err := store.Delete(staged.Key()); err != nil {
		t.Fatal("missing delete is not idempotent")
	}
}

func TestLocalStageLimitsAndCleanup(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stage(context.Background(), strings.NewReader(""), 1); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty stage error = %v", err)
	}
	if _, err := store.Stage(context.Background(), strings.NewReader("ab"), 1); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized stage error = %v", err)
	}
	staged, err := store.Stage(context.Background(), strings.NewReader("x"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Abort(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".stage-") {
			t.Fatal("staged object remains after abort")
		}
	}
}

func TestLocalObjectPathsRejectTraversalAndSymlinks(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "../" + strings.Repeat("a", 32), strings.Repeat("A", 32), strings.Repeat("a", 31)} {
		if _, err := store.path(key); err == nil {
			t.Fatalf("invalid key %q accepted", key)
		}
	}
	staged, err := store.Stage(context.Background(), bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Commit(); err != nil {
		t.Fatal(err)
	}
	path, err := store.path(staged.Key())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(staged.Key()); err == nil {
		t.Fatal("symlink object opened")
	}
	if err := store.Delete(staged.Key()); err == nil {
		t.Fatal("symlink object deleted")
	}
}

func TestLocalStageWriteAndFinalizeFailuresCleanUp(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stage(context.Background(), failingReader{}, 10); err == nil {
		t.Fatal("failing source staged")
	}
	entries, err := os.ReadDir(store.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".stage-") {
			t.Fatal("failed stage temp remains")
		}
	}

	staged, err := store.Stage(context.Background(), strings.NewReader("x"), 1)
	if err != nil {
		t.Fatal(err)
	}
	final, err := store.path(staged.Key())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/tmp", filepath.Dir(final)); err != nil {
		t.Fatal(err)
	}
	if err := staged.Commit(); err == nil {
		t.Fatal("symlink shard finalized")
	}
	if err := staged.Abort(); err != nil {
		t.Fatal(err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("write failure") }

func TestLocalStagesExact64MiB(t *testing.T) {
	store, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const size = 64 << 20
	staged, err := store.Stage(context.Background(), io.LimitReader(zeroReader{}, size), size)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Size() != size {
		t.Fatalf("size = %d", staged.Size())
	}
	if err := staged.Abort(); err != nil {
		t.Fatal(err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = 0
	}
	return len(value), nil
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Fatalf("mode %o = %o", want, info.Mode().Perm())
	}
}
