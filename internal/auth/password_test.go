package auth_test

import (
	"testing"

	"github.com/ybordag/cambium/internal/auth"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if err := auth.CheckPassword(hash, "correct-horse-battery-staple"); err != nil {
		t.Errorf("CheckPassword: expected nil for correct password, got %v", err)
	}

	if err := auth.CheckPassword(hash, "wrong-password"); err == nil {
		t.Error("CheckPassword: expected error for wrong password, got nil")
	}
}
