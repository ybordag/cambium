package api

import (
	"net/http"
	"strings"

	"github.com/ybordag/cambium/internal/auth"
)

// RequireAuth wraps a handler, verifying the Authorization: Bearer token.
// On success, the user_id from the JWT sub claim is stored in the request context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		userID, err := auth.VerifyAccessToken(tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		next.ServeHTTP(w, withUserID(r, userID))
	})
}
