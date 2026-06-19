package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires all routes and returns the root handler.
// The pool is passed through for handlers that need DB access (Phase 2+).
func NewRouter(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)

	return mux
}
