package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID                   string
	Email                string
	PasswordHash         string
	PreferredProvider    *string
	PreferredModel       *string
	EncryptedGeminiKey   *string
	EncryptedOpenAIKey   *string
	EncryptedAnthropicKey *string
}

var ErrNotFound = errors.New("not found")
var ErrEmailTaken = errors.New("email already registered")

// InsertUser creates a new user row and returns the generated UUID.
func InsertUser(ctx context.Context, pool *pgxpool.Pool, email, passwordHash string) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO cambium.users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id`,
		email, passwordHash,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrEmailTaken
		}
		return "", fmt.Errorf("insert user: %w", err)
	}
	return id, nil
}

// GetUserByEmail returns the user with the given email, or ErrNotFound.
func GetUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx,
		`SELECT id, email, password_hash,
		        preferred_provider, preferred_model,
		        encrypted_gemini_key, encrypted_openai_key, encrypted_anthropic_key
		 FROM cambium.users WHERE email = $1`,
		email,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash,
		&u.PreferredProvider, &u.PreferredModel,
		&u.EncryptedGeminiKey, &u.EncryptedOpenAIKey, &u.EncryptedAnthropicKey,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetUserByID returns the user with the given UUID, or ErrNotFound.
func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx,
		`SELECT id, email, password_hash,
		        preferred_provider, preferred_model,
		        encrypted_gemini_key, encrypted_openai_key, encrypted_anthropic_key
		 FROM cambium.users WHERE id = $1`,
		id,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash,
		&u.PreferredProvider, &u.PreferredModel,
		&u.EncryptedGeminiKey, &u.EncryptedOpenAIKey, &u.EncryptedAnthropicKey,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// SetProviderKey updates the encrypted key column for the given provider.
// provider must be "gemini", "openai", or "anthropic".
func SetProviderKey(ctx context.Context, pool *pgxpool.Pool, userID, provider, encryptedKey string) error {
	col, err := providerColumn(provider)
	if err != nil {
		return err
	}
	_, execErr := pool.Exec(ctx,
		fmt.Sprintf(`UPDATE cambium.users SET %s = $1 WHERE id = $2`, col),
		encryptedKey, userID,
	)
	return execErr
}

// ClearProviderKey nulls out the encrypted key column for the given provider.
func ClearProviderKey(ctx context.Context, pool *pgxpool.Pool, userID, provider string) error {
	col, err := providerColumn(provider)
	if err != nil {
		return err
	}
	_, execErr := pool.Exec(ctx,
		fmt.Sprintf(`UPDATE cambium.users SET %s = NULL WHERE id = $1`, col),
		userID,
	)
	return execErr
}

func providerColumn(provider string) (string, error) {
	switch provider {
	case "gemini":
		return "encrypted_gemini_key", nil
	case "openai":
		return "encrypted_openai_key", nil
	case "anthropic":
		return "encrypted_anthropic_key", nil
	default:
		return "", fmt.Errorf("unknown provider %q: must be gemini, openai, or anthropic", provider)
	}
}

// UpdateProfile sets preferred_provider and/or preferred_model on the user row.
// Pass nil for fields that should not change.
func UpdateProfile(ctx context.Context, pool *pgxpool.Pool, userID string, provider, model *string) error {
	if provider == nil && model == nil {
		return nil
	}
	if provider != nil && model != nil {
		_, err := pool.Exec(ctx,
			`UPDATE cambium.users SET preferred_provider = $1, preferred_model = $2 WHERE id = $3`,
			*provider, *model, userID)
		return err
	}
	if provider != nil {
		_, err := pool.Exec(ctx,
			`UPDATE cambium.users SET preferred_provider = $1 WHERE id = $2`,
			*provider, userID)
		return err
	}
	_, err := pool.Exec(ctx,
		`UPDATE cambium.users SET preferred_model = $1 WHERE id = $2`,
		*model, userID)
	return err
}

// UpdatePassword replaces the stored bcrypt hash for the given user.
func UpdatePassword(ctx context.Context, pool *pgxpool.Pool, userID, newHash string) error {
	_, err := pool.Exec(ctx,
		`UPDATE cambium.users SET password_hash = $1 WHERE id = $2`,
		newHash, userID)
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && fmt.Sprintf("%v", err) != "" &&
		containsStr(err.Error(), "23505")
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findStr(s, sub))
}

func findStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
