package share

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
)

func TestFileCreateCompensatesAfterDatabaseFailure(t *testing.T) {
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := objectstore.OpenLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	service := NewWithStore(db, store, time.Now)
	staged, err := service.StageFile(context.Background(), strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	key := staged.Key()
	if _, err := db.Exec("DROP TABLE file_payloads"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.CreateFile(context.Background(), "x.bin", staged, nil); err == nil {
		t.Fatal("File create succeeded without payload table")
	}
	if _, err := store.Open(key); err == nil {
		t.Fatal("object remained after failed DB creation")
	}
}

func TestFileDeleteCleanupFailureDoesNotResurrectShare(t *testing.T) {
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := objectstore.OpenLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	service := NewWithStore(db, store, time.Now)
	staged, err := service.StageFile(context.Background(), strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	value, token, err := service.CreateFile(context.Background(), "x.bin", staged, nil)
	if err != nil {
		t.Fatal(err)
	}
	failing := NewWithStore(db, deleteFailureStore{Store: store}, time.Now)
	err = failing.Delete(context.Background(), value.ID, token.Reveal())
	var cleanup *CleanupError
	if !errors.As(err, &cleanup) || cleanup.StorageKey != value.File.StorageKey {
		t.Fatalf("delete error = %v", err)
	}
	if _, err := service.Get(context.Background(), value.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted File Share loaded: %v", err)
	}
	if _, err := store.Open(value.File.StorageKey); err != nil {
		t.Fatal("orphan object was unexpectedly removed")
	}
}

type deleteFailureStore struct{ objectstore.Store }

func (store deleteFailureStore) Delete(string) error { return errors.New("delete failed") }
