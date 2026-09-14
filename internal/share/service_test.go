package share

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
)

func TestCreateIsAtomicWhenPayloadInsertFails(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	service := New(db, func() time.Time { return now })
	id, err := capability.GenerateShareID()
	if err != nil {
		t.Fatal("generate Share ID")
	}
	token, err := capability.GenerateOwnerToken()
	if err != nil {
		t.Fatal("generate owner token")
	}
	value := Share{
		ID:          id,
		PayloadKind: domain.PayloadText,
		PrivacyMode: domain.PrivacyStandard,
		Text:        &Text{Format: domain.TextFormat("INVALID"), Content: "content"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := service.insert(context.Background(), value, token.Verifier()); err == nil {
		t.Fatal("invalid payload insertion unexpectedly succeeded")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM shares WHERE id = ?", id.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed payload insertion left orphan metadata")
	}
}

func TestExpiredShareCannotBeRevived(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	service := New(db, func() time.Time { return now })
	expires := now.Add(time.Hour)
	created, token, err := service.Create(context.Background(), CreateInput{
		Text:      Text{Format: domain.TextPlain, Content: "original"},
		ExpiresAt: &expires,
	})
	if err != nil {
		t.Fatal(err)
	}
	now = expires
	future := now.Add(time.Hour)
	_, err = service.Update(context.Background(), created.ID, token.Reveal(), Patch{ExpirationSet: true, ExpiresAt: &future})
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("Update error = %v, want ErrExpired", err)
	}
	var storedExpiration int64
	if err := db.QueryRow("SELECT expires_at FROM shares WHERE id = ?", created.ID.String()).Scan(&storedExpiration); err != nil {
		t.Fatal(err)
	}
	if storedExpiration != expires.UnixMilli() {
		t.Fatal("expired Share expiration changed")
	}
}

func TestCreateNormalizesTimestamps(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 14, 3, 0, 0, 123456789, time.FixedZone("offset", 3600))
	service := New(db, func() time.Time { return now })
	expires := now.Add(time.Hour + 999*time.Microsecond)
	created, _, err := service.Create(context.Background(), CreateInput{
		Text:      Text{Format: domain.TextPlain, Content: "content"},
		ExpiresAt: &expires,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedAt.Location() != time.UTC || created.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 {
		t.Fatal("created_at was not normalized to UTC milliseconds")
	}
	if created.ExpiresAt == nil || created.ExpiresAt.Location() != time.UTC || created.ExpiresAt.Nanosecond()%int(time.Millisecond) != 0 {
		t.Fatal("expires_at was not normalized to UTC milliseconds")
	}
}
