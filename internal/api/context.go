package api

import (
	"context"
	"net/http"
)

type contextKey string

const userIDKey contextKey = "user_id"

func withUserID(r *http.Request, userID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
}

// UserIDFromContext extracts the authenticated user ID set by the JWT middleware.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && id != ""
}
