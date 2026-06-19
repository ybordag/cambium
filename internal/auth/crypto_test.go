package auth_test

import (
	"testing"

	"github.com/ybordag/cambium/internal/auth"
)

func TestEncryptDecryptKey(t *testing.T) {
	t.Setenv("CAMBIUM_ENCRYPTION_KEY", "12345678901234567890123456789012") // 32 bytes

	plaintext := "sk-proj-supersecretapikey"

	encrypted, err := auth.EncryptKey(plaintext)
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}

	if encrypted == plaintext {
		t.Error("encrypted value should not equal plaintext")
	}

	decrypted, err := auth.DecryptKey(encrypted)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptKey_NonceIsRandom(t *testing.T) {
	t.Setenv("CAMBIUM_ENCRYPTION_KEY", "12345678901234567890123456789012")

	a, _ := auth.EncryptKey("same-plaintext")
	b, _ := auth.EncryptKey("same-plaintext")

	if a == b {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestEncryptionKey_WrongLength(t *testing.T) {
	t.Setenv("CAMBIUM_ENCRYPTION_KEY", "tooshort")
	_, err := auth.EncryptKey("anything")
	if err == nil {
		t.Error("expected error for wrong-length key, got nil")
	}
}
