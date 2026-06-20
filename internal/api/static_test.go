package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// setupStaticDir writes a minimal dist/ tree (index.html + a JS asset) to a
// temp dir and points STATIC_DIR at it for the duration of the test.
func setupStaticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app shell</html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "index.abc123.js"), []byte("console.log('verdant');"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	t.Setenv("STATIC_DIR", dir)
	return dir
}

func TestStaticFileHandler_RootServesIndexHTML(t *testing.T) {
	setupStaticDir(t)
	handler := staticFileHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() != "<html>app shell</html>" {
		t.Errorf("got body %q", rec.Body.String())
	}
}

func TestStaticFileHandler_UnknownPathFallsBackToIndexHTML(t *testing.T) {
	setupStaticDir(t)
	handler := staticFileHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/tasks", nil)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() != "<html>app shell</html>" {
		t.Errorf("got body %q, want index.html fallback", rec.Body.String())
	}
}

func TestStaticFileHandler_KnownAssetServedDirectly(t *testing.T) {
	setupStaticDir(t)
	handler := staticFileHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/index.abc123.js", nil)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() != "console.log('verdant');" {
		t.Errorf("got body %q, want the JS asset", rec.Body.String())
	}
}

func TestStaticFileHandler_PathTraversalCannotEscapeStaticDir(t *testing.T) {
	setupStaticDir(t)
	handler := staticFileHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/../../../../etc/passwd", nil)
	handler(rec, req)

	// Either falls back to index.html (200) or 404s — must never serve a
	// file from outside STATIC_DIR.
	if rec.Code == http.StatusOK && rec.Body.String() != "<html>app shell</html>" {
		t.Errorf("path traversal served unexpected content: %q", rec.Body.String())
	}
}

func TestStaticFileHandler_DefaultsToDistWhenUnset(t *testing.T) {
	os.Unsetenv("STATIC_DIR")
	handler := staticFileHandler()
	if handler == nil {
		t.Fatal("expected non-nil handler with default STATIC_DIR")
	}
}

// TestRouter_StaticFallbackDoesNotShadowAPIRoutes proves the catch-all "/"
// registration never intercepts a more specific registered pattern, even
// though static.go is wired in after every other route in NewRouter.
func TestRouter_StaticFallbackDoesNotShadowAPIRoutes(t *testing.T) {
	setupStaticDir(t)
	router := NewRouter(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() == "<html>app shell</html>" {
		t.Error("GET /health was served by the static fallback instead of healthHandler")
	}
}

func TestRouter_UnknownPathFallsBackToStaticHandler(t *testing.T) {
	setupStaticDir(t)
	router := NewRouter(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/tasks", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() != "<html>app shell</html>" {
		t.Errorf("got body %q, want index.html fallback", rec.Body.String())
	}
}

// TestRouter_APIRouteNotShadowedByStaticFallback is the literal acceptance
// criterion from #21: GET /api/v1/garden/profile must still reach the real
// handler. It reaches RequireAuth (401, missing token) rather than the
// static index.html (200) — proving the request was routed to the API
// handler chain, not served as a frontend asset.
func TestRouter_APIRouteNotShadowedByStaticFallback(t *testing.T) {
	setupStaticDir(t)
	router := NewRouter(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/garden/profile", nil)
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK && rec.Body.String() == "<html>app shell</html>" {
		t.Error("GET /api/v1/garden/profile was served by the static fallback instead of the API handler")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401 (RequireAuth rejecting the missing token)", rec.Code)
	}
}

// TestRouter_SwaggerDocsNotShadowedByStaticFallback is the other literal
// acceptance criterion from #21: swagger UI must remain reachable at
// /docs/index.html.
func TestRouter_SwaggerDocsNotShadowedByStaticFallback(t *testing.T) {
	setupStaticDir(t)
	router := NewRouter(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/index.html", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() == "<html>app shell</html>" {
		t.Error("GET /docs/index.html was served by the static fallback instead of swagger UI")
	}
}

func TestStaticFileHandler_DirectoryPathFallsBackToIndexHTML(t *testing.T) {
	setupStaticDir(t)
	handler := staticFileHandler()

	rec := httptest.NewRecorder()
	// "assets" exists as a directory, not a file — must not attempt to
	// serve/list it; must fall back to index.html like any other miss.
	req := httptest.NewRequest(http.MethodGet, "/assets", nil)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if rec.Body.String() != "<html>app shell</html>" {
		t.Errorf("got body %q, want index.html fallback for a directory path", rec.Body.String())
	}
}

func TestStaticFileHandler_MissingStaticDirReturns404NotPanic(t *testing.T) {
	t.Setenv("STATIC_DIR", "/nonexistent/path/does-not-exist")
	handler := staticFileHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404 when STATIC_DIR and its index.html don't exist", rec.Code)
	}
}
