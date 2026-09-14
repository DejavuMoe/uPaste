package share

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
)

func TestFileCreateCompensatesCommitPublicationFailure(t *testing.T) {
	for _, deleteFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "cleanup succeeds", true: "cleanup fails"}[deleteFails], func(t *testing.T) {
			db, err := database.Open(context.Background(), t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			store := &publishedFailureStore{deleteErr: map[bool]error{false: nil, true: errors.New("cleanup failed")}[deleteFails]}
			service := NewWithStore(db, store, time.Now)
			_, _, err = service.CreateFile(context.Background(), "x.bin", &publishedFailureStaged{store: store}, nil)
			if !errors.Is(err, errPublished) {
				t.Fatalf("CreateFile error = %v", err)
			}
			if deleteFails && !strings.Contains(err.Error(), "cleanup failed") {
				t.Fatalf("cleanup failure lost: %v", err)
			}
			if len(store.deleted) != 1 || store.deleted[0] != store.key || (!deleteFails && store.published) {
				t.Fatalf("compensation = %+v published=%t", store.deleted, store.published)
			}
			var shares, files int
			if err := db.QueryRow("SELECT count(*) FROM shares WHERE payload_kind = 'FILE'").Scan(&shares); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT count(*) FROM file_payloads").Scan(&files); err != nil {
				t.Fatal(err)
			}
			if shares != 0 || files != 0 {
				t.Fatalf("uncommitted rows = %d/%d", shares, files)
			}
		})
	}
}

var errPublished = errors.New("published then failed")

type publishedFailureStore struct {
	key       string
	published bool
	deleted   []string
	deleteErr error
}

func (store *publishedFailureStore) Stage(context.Context, io.Reader, int64) (objectstore.Staged, error) {
	return nil, errors.New("unused")
}
func (store *publishedFailureStore) Open(string) (objectstore.ReadSeekCloser, error) {
	return nil, errors.New("unused")
}
func (store *publishedFailureStore) Delete(key string) error {
	store.deleted = append(store.deleted, key)
	if store.deleteErr == nil {
		store.published = false
	}
	return store.deleteErr
}

type publishedFailureStaged struct{ store *publishedFailureStore }

func (staged *publishedFailureStaged) Key() string {
	staged.store.key = strings.Repeat("a", 32)
	return staged.store.key
}
func (*publishedFailureStaged) Size() int64               { return 1 }
func (*publishedFailureStaged) SHA256() [sha256.Size]byte { return sha256.Sum256([]byte("x")) }
func (*publishedFailureStaged) Sniff() []byte             { return []byte("x") }
func (staged *publishedFailureStaged) Commit() error {
	staged.store.published = true
	return errPublished
}
func (*publishedFailureStaged) Abort() error { return nil }

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
