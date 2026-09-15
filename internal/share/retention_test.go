package share

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
)

func TestPublicRetentionCreationAndUpdateHorizon(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, func() time.Time { return now })
	service.SetRetentionPolicy(RetentionPolicy{Public: true, DefaultTTL: 24 * time.Hour, MaxTTL: 168 * time.Hour})

	// nil expiration defaults to the public default TTL.
	created, token, err := service.Create(context.Background(), CreateInput{Text: Text{Format: domain.TextPlain, Content: "public"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ExpiresAt == nil || !created.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("default expiration = %v, want %v", created.ExpiresAt, now.Add(24*time.Hour))
	}

	encrypted, _, err := service.CreateEncrypted(context.Background(), EncryptedCreateInput{EncryptedText: EncryptedText{
		Protocol: EncryptedTextProtocolV1, Nonce: make([]byte, 12), Ciphertext: make([]byte, 19),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if encrypted.ExpiresAt == nil || !encrypted.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("encrypted default expiration = %v", encrypted.ExpiresAt)
	}

	// Requested expiration beyond the creation horizon is rejected.
	tooLong := now.Add(169 * time.Hour)
	if _, _, err := service.Create(context.Background(), CreateInput{Text: Text{Format: domain.TextPlain, Content: "too long"}, ExpiresAt: &tooLong}); !errors.Is(err, ErrRetention) {
		t.Fatalf("over-horizon create error = %v", err)
	}

	// Shortening works.
	short := now.Add(2 * time.Hour)
	if _, err := service.Update(context.Background(), created.ID, token.Reveal(), Patch{ExpirationSet: true, ExpiresAt: &short}); err != nil {
		t.Fatalf("shorten expiration: %v", err)
	}

	// Extending within the original horizon works.
	within := created.CreatedAt.Add(167 * time.Hour)
	updated, err := service.Update(context.Background(), created.ID, token.Reveal(), Patch{ExpirationSet: true, ExpiresAt: &within})
	if err != nil {
		t.Fatalf("within-horizon extension: %v", err)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(within) {
		t.Fatalf("within-horizon expiration = %v", updated.ExpiresAt)
	}

	// Repeated PATCH cannot push past created_at + max TTL.
	beyond := created.CreatedAt.Add(169 * time.Hour)
	if _, err := service.Update(context.Background(), created.ID, token.Reveal(), Patch{ExpirationSet: true, ExpiresAt: &beyond}); !errors.Is(err, ErrRetention) {
		t.Fatalf("over-horizon patch error = %v", err)
	}

	// Clearing expiration is forbidden in public mode.
	if _, err := service.Update(context.Background(), created.ID, token.Reveal(), Patch{ExpirationSet: true, ExpiresAt: nil}); !errors.Is(err, ErrRetention) {
		t.Fatalf("nil patch expiration error = %v", err)
	}
}

func TestPrivateModeRetentionRemainsUnboundedOptional(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, func() time.Time { return now })

	created, _, err := service.Create(context.Background(), CreateInput{Text: Text{Format: domain.TextPlain, Content: "private"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ExpiresAt != nil {
		t.Fatalf("private nil expiration = %v, want nil", created.ExpiresAt)
	}
	veryLong := now.Add(10000 * time.Hour)
	if _, _, err := service.Create(context.Background(), CreateInput{Text: Text{Format: domain.TextPlain, Content: "long"}, ExpiresAt: &veryLong}); err != nil {
		t.Fatalf("private long expiration rejected: %v", err)
	}
}

func TestPublicRetentionAppliesToFileCreation(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
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
	defer store.Close()
	service := NewWithStore(db, store, func() time.Time { return now })
	service.SetRetentionPolicy(RetentionPolicy{Public: true, DefaultTTL: 24 * time.Hour, MaxTTL: 168 * time.Hour})

	staged, err := service.StageFile(context.Background(), strings.NewReader("file body"))
	if err != nil {
		t.Fatal(err)
	}
	value, _, err := service.CreateFile(context.Background(), "retained.bin", staged, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.ExpiresAt == nil || !value.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("public File default expiration = %v", value.ExpiresAt)
	}
}
