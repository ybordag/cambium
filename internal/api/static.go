package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// staticFileHandler serves the built Verdant Pages frontend from STATIC_DIR
// (default "./dist"). Any request path that doesn't correspond to an
// existing file falls back to index.html, so client-side routing (React
// Router) works for any unknown path.
//
// Registered as the catch-all "/" pattern in NewRouter. Go's ServeMux always
// prefers the most specific matching pattern regardless of registration
// order, so this never shadows /api/v1/*, /auth/*, /health, or /docs/*.
func staticFileHandler() http.HandlerFunc {
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./dist"
	}
	fileServer := http.FileServer(http.Dir(staticDir))

	return func(w http.ResponseWriter, r *http.Request) {
		// path.Clean on a URL path (always "/"-rooted) resolves ".." segments
		// lexically and can never escape above the root — safe to join below.
		requested := filepath.Join(staticDir, path.Clean(r.URL.Path))
		if info, err := os.Stat(requested); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}
