package maintenance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
	"github.com/DejavuMoe/uPaste/internal/share"
)

func TestRunOncePurgesExpiredPayloads(t *testing.T) {
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
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
	service := share.NewWithStore(db, store, func() time.Time { return now.Add(-time.Hour) })
	expires := now
	text, _, err := service.Create(context.Background(), share.CreateInput{Text: share.Text{Format: domain.TextPlain, Content: "x"}, ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	enc, _, err := service.CreateEncrypted(context.Background(), share.EncryptedCreateInput{EncryptedText: share.EncryptedText{Protocol: share.EncryptedTextProtocolV1, Nonce: make([]byte, 12), Ciphertext: make([]byte, 19)}, ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := service.StageFile(context.Background(), strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := service.CreateFile(context.Background(), "x", staged, &expires)
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(db, store).RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Purged != 3 || result.FileObjectsDeleted != 1 {
		t.Fatalf("result = %+v", result)
	}
	for _, id := range []string{text.ID.String(), enc.ID.String(), file.ID.String()} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM shares WHERE id = ?", id).Scan(&count); err != nil || count != 0 {
			t.Fatalf("share %s remains: %d/%v", id, count, err)
		}
	}
	if _, err := store.Open(file.File.StorageKey); err == nil {
		t.Fatal("File object remains")
	}
}

func TestRunOnceLeavesUnexpiredAndBoundsBacklog(t *testing.T) {
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := objectstore.OpenLocal(dataDir)
	service := share.NewWithStore(db, store, func() time.Time { return now.Add(-time.Hour) })
	future := now.Add(time.Hour)
	retained, _, err := service.Create(context.Background(), share.CreateInput{Text: share.Text{Format: domain.TextPlain, Content: "x"}, ExpiresAt: &future})
	if err != nil {
		t.Fatal(err)
	}
	for range MaxPerRun + 1 {
		if _, _, err := service.Create(context.Background(), share.CreateInput{Text: share.Text{Format: domain.TextPlain, Content: "x"}, ExpiresAt: &now}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := New(db, store).RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Purged != MaxPerRun || !result.Backlog {
		t.Fatalf("result = %+v", result)
	}
	if _, err := service.Get(context.Background(), retained.ID); err != nil {
		t.Fatal(err)
	}
}

type deleteFailureStore struct{ objectstore.Store }

func (store deleteFailureStore) Delete(string) error { return errors.New("delete failed") }
func TestPurgeCleanupFailureStillPurges(t *testing.T) {
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()
	db, _ := database.Open(context.Background(), dataDir)
	defer db.Close()
	local, _ := objectstore.OpenLocal(dataDir)
	shares := share.NewWithStore(db, local, func() time.Time { return now.Add(-time.Hour) })
	staged, _ := shares.StageFile(context.Background(), strings.NewReader("x"))
	file, _, err := shares.CreateFile(context.Background(), "x", staged, &now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(db, deleteFailureStore{local}).RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if result.FileCleanupFailures != 1 {
		t.Fatal(result)
	}
	if _, err := shares.Get(context.Background(), file.ID); !errors.Is(err, share.ErrNotFound) {
		t.Fatal(err)
	}
}
