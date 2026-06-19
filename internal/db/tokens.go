package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// GenerateRefreshToken produces a cryptographically random token string.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of a token for storage.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// InsertRefreshToken stores a hashed refresh token and returns its UUID.
func InsertRefreshToken(ctx context.Context, pool *pgxpool.Pool, userID, tokenHash string, expiresAt time.Time) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO cambium.refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		userID, tokenHash, expiresAt,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert refresh token: %w", err)
	}
	return id, nil
}

// GetRefreshToken returns the token row for the given hash, or ErrNotFound.
func GetRefreshToken(ctx context.Context, pool *pgxpool.Pool, tokenHash string) (*RefreshToken, error) {
	t := &RefreshToken{}
	err := pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at
		 FROM cambium.refresh_tokens
		 WHERE token_hash = $1`,
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return t, nil
}

// RevokeRefreshToken sets revoked_at to now for the given token row ID.
func RevokeRefreshToken(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx,
		`UPDATE cambium.refresh_tokens SET revoked_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

// RevokeAllUserTokens revokes every active refresh token for a user (logout all devices).
func RevokeAllUserTokens(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE cambium.refresh_tokens
		 SET revoked_at = NOW()
		 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	return err
}
