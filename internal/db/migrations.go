package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate creates the cambium schema and its tables if they do not already exist.
// Safe to call on every startup — all statements are idempotent.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	steps := []struct {
		name string
		sql  string
	}{
		{
			name: "create cambium schema",
			sql:  `CREATE SCHEMA IF NOT EXISTS cambium`,
		},
		{
			name: "create users table",
			sql: `
				CREATE TABLE IF NOT EXISTS cambium.users (
					id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					email                   TEXT UNIQUE NOT NULL,
					password_hash           TEXT NOT NULL,
					preferred_provider      TEXT,
					preferred_model         TEXT,
					encrypted_gemini_key    TEXT,
					encrypted_openai_key    TEXT,
					encrypted_anthropic_key TEXT,
					created_at              TIMESTAMP NOT NULL DEFAULT NOW()
				)`,
		},
		{
			name: "create refresh_tokens table",
			sql: `
				CREATE TABLE IF NOT EXISTS cambium.refresh_tokens (
					id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id     UUID NOT NULL REFERENCES cambium.users(id),
					token_hash  TEXT NOT NULL,
					expires_at  TIMESTAMP NOT NULL,
					created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
					revoked_at  TIMESTAMP
				)`,
		},
	}

	for _, step := range steps {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			return fmt.Errorf("migration %q: %w", step.name, err)
		}
	}

	return nil
}
