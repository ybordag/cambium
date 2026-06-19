package auth_test

import (
	"testing"

	"github.com/ybordag/cambium/internal/auth"
)

func TestIssueAndVerifyAccessToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-that-is-at-least-32-bytes!!")

	token, err := auth.IssueAccessToken("user-123")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	userID, err := auth.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("got user_id %q, want %q", userID, "user-123")
	}
}

func TestVerifyAccessToken_InvalidSignature(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-that-is-at-least-32-bytes!!")

	_, err := auth.VerifyAccessToken("this.is.not.a.valid.jwt")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}

func TestVerifyAccessToken_WrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-that-is-at-least-32-bytes!!")
	token, _ := auth.IssueAccessToken("user-123")

	t.Setenv("JWT_SECRET", "different-secret-also-at-least-32-bytes!")
	_, err := auth.VerifyAccessToken(token)
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
}

func TestJWTSecret_TooShort(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	_, err := auth.IssueAccessToken("user-123")
	if err == nil {
		t.Error("expected error for short secret, got nil")
	}
}
