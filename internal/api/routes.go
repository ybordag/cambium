package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires all routes and returns the root handler.
func NewRouter(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()

	ah := &authHandler{pool: pool}
	kh := &keysHandler{pool: pool}

	// Public
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /auth/register", ah.register)
	mux.HandleFunc("POST /auth/login", ah.login)
	mux.HandleFunc("POST /auth/refresh", ah.refresh)
	mux.HandleFunc("POST /auth/logout", ah.logout)

	// Protected — require valid JWT
	mux.Handle("GET /auth/session", RequireAuth(http.HandlerFunc(ah.session)))
	mux.Handle("PUT /api/v1/auth/keys", RequireAuth(http.HandlerFunc(kh.set)))
	mux.Handle("GET /api/v1/auth/keys", RequireAuth(http.HandlerFunc(kh.list)))
	mux.Handle("DELETE /api/v1/auth/keys/{provider}", RequireAuth(http.HandlerFunc(kh.delete)))

	return mux
}
