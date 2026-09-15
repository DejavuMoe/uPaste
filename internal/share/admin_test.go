package share

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
)

func TestAdminListQueryIsPayloadFree(t *testing.T) {
	for _, forbidden := range []string{
		", p.content", ", e.nonce", ", e.ciphertext", ", f.storage_key", ", f.content_sha256", "owner_token_verifier",
	} {
		if strings.Contains(adminListQuery, forbidden) {
			t.Fatalf("admin list query selects payload column %q:\n%s", forbidden, adminListQuery)
		}
	}
	for _, required := range []string{
		"length(CAST(p.content AS BLOB))",
		"length(e.ciphertext)",
		"f.size_bytes",
		"payload_bytes",
		"f.original_filename",
		"f.detected_media_type",
	} {
		if !strings.Contains(adminListQuery, required) {
			t.Fatalf("admin list query missing required expression %q:\n%s", required, adminListQuery)
		}
	}
}

func TestAdminListMetadataBytesAndCursor(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	current := now
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
	service := NewWithStore(db, store, func() time.Time { return current })
	ctx := context.Background()

	largeText := strings.Repeat("x", 256*1024)
	current = current.Add(time.Millisecond)
	text, _, err := service.Create(ctx, CreateInput{Text: Text{Format: domain.TextPlain, Content: largeText}})
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Millisecond)
	largeCipher := make([]byte, 256*1024+16)
	encrypted, _, err := service.CreateEncrypted(ctx, EncryptedCreateInput{EncryptedText: EncryptedText{
		Protocol: EncryptedTextProtocolV1, Nonce: make([]byte, 12), Ciphertext: largeCipher,
	}})
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Millisecond)
	staged, err := service.StageFile(ctx, strings.NewReader(strings.Repeat("f", 128*1024)))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := service.CreateFile(ctx, "large.bin", staged, nil)
	if err != nil {
		t.Fatal(err)
	}

	items, next, err := service.AdminList(ctx, AdminFilter{Limit: 2, Sort: "oldest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || next == "" {
		t.Fatalf("first page = %d items, next=%q", len(items), next)
	}
	if items[0].ID != text.ID || items[0].PayloadBytes != int64(len(largeText)) || items[0].FileFilename != "" {
		t.Fatalf("first metadata item = %+v", items[0])
	}
	if items[1].ID != encrypted.ID || items[1].PayloadBytes != int64(len(largeCipher)) {
		t.Fatalf("second metadata item = %+v", items[1])
	}
	second, _, err := service.AdminList(ctx, AdminFilter{Limit: 2, Sort: "oldest", Cursor: next})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ID != file.ID || second[0].PayloadBytes != int64(128*1024) {
		t.Fatalf("second page = %+v", second)
	}
	if second[0].FileFilename != "large.bin" || second[0].FileMediaType == "" {
		t.Fatalf("File metadata item = %+v", second[0])
	}

	exact, _, err := service.AdminList(ctx, AdminFilter{Limit: 10, ExactID: encrypted.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(exact) != 1 || exact[0].ID != encrypted.ID || exact[0].PayloadBytes != int64(len(largeCipher)) {
		t.Fatalf("exact metadata item = %+v", exact)
	}
}
