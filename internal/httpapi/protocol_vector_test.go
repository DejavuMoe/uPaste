package httpapi

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"
)

func TestEncryptedTextProtocolKnownAnswerWithGoAESGCM(t *testing.T) {
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index)
	}
	nonce := make([]byte, 12)
	for index := range nonce {
		nonce[index] = byte(0xa0 + index)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	envelope := append([]byte{1, 2}, []byte("uPaste protocol v1 ✓")...)
	ciphertext := gcm.Seal(nil, nonce, envelope, []byte("uPaste:encrypted-text:v1"))
	const expected = "5xoJfSS4dtpCFfW8cxWjsRyMLyGyVd7_0GUFs_5Om4VV275yEuMUJQ"
	if got := base64.RawURLEncoding.EncodeToString(ciphertext); got != expected {
		t.Fatalf("ciphertext = %q, want known-answer vector", got)
	}
}
