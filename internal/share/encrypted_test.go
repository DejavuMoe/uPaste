package share

import (
	"context"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
)

func TestEncryptedCreateAndLoadUsesExclusivePayloadModel(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, time.Now)
	nonce := make([]byte, EncryptedNonceBytes)
	ciphertext := make([]byte, MinEncryptedCipherBytes)
	created, _, err := service.CreateEncrypted(context.Background(), EncryptedCreateInput{EncryptedText: EncryptedText{
		Protocol: EncryptedTextProtocolV1, Nonce: nonce, Ciphertext: ciphertext,
	}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Text != nil || loaded.EncryptedText == nil || loaded.EncryptedText.Protocol != EncryptedTextProtocolV1 {
		t.Fatal("encrypted Share exposed the wrong payload model")
	}
}

func TestEncryptedCreateIsAtomicWhenPayloadInsertFails(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, time.Now)
	id, err := capability.GenerateShareID()
	if err != nil {
		t.Fatal(err)
	}
	token, err := capability.GenerateOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	value := Share{
		ID: id, PayloadKind: domain.PayloadText, PrivacyMode: domain.PrivacyEncrypted,
		EncryptedText: &EncryptedText{Protocol: EncryptedTextProtocolV1, Nonce: make([]byte, 12), Ciphertext: make([]byte, 18)},
		CreatedAt:     now, UpdatedAt: now,
	}
	if err := service.insert(context.Background(), value, token.Verifier()); err == nil {
		t.Fatal("invalid encrypted payload insertion unexpectedly succeeded")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM shares WHERE id = ?", id.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed encrypted payload insertion left Share metadata")
	}
}
