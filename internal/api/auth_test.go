package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ybordag/cambium/internal/api"
	"github.com/ybordag/cambium/internal/db"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-jwt-secret-at-least-32-bytes!!")
	os.Setenv("CAMBIUM_ENCRYPTION_KEY", "12345678901234567890123456789012")
	os.Exit(m.Run())
}

// newTestPool connects to Postgres and skips the test if unavailable.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Connect(context.Background())
	if err != nil {
		t.Skipf("skipping: no test database available (%v)", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM cambium.refresh_tokens`)
		pool.Exec(context.Background(), `DELETE FROM cambium.users`)
		pool.Close()
	})
	return pool
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return api.NewRouter(newTestPool(t))
}

func doRequest(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func doRequestWithToken(t *testing.T, srv http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func registerAndGetToken(t *testing.T, srv http.Handler, email string) string {
	t.Helper()
	body := `{"email":"` + email + `","password":"pw12345678"}`
	resp := doRequest(t, srv, "POST", "/auth/register", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("register %s: got %d — %s", email, resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	return out["access_token"]
}

// --- tests ---

func TestRegister(t *testing.T) {
	srv := newTestServer(t)

	resp := doRequest(t, srv, "POST", "/auth/register", `{"email":"alice@example.com","password":"hunter2hunter2"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("register: got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["access_token"] == "" {
		t.Error("expected access_token in response")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	srv := newTestServer(t)

	doRequest(t, srv, "POST", "/auth/register", `{"email":"dup@example.com","password":"hunter2"}`)
	resp := doRequest(t, srv, "POST", "/auth/register", `{"email":"dup@example.com","password":"hunter2"}`)
	if resp.Code != http.StatusConflict {
		t.Errorf("duplicate register: got %d, want 409", resp.Code)
	}
}

func TestLogin_CorrectPassword(t *testing.T) {
	srv := newTestServer(t)

	doRequest(t, srv, "POST", "/auth/register", `{"email":"bob@example.com","password":"correct-pw"}`)
	resp := doRequest(t, srv, "POST", "/auth/login", `{"email":"bob@example.com","password":"correct-pw"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("login: got %d, want 200", resp.Code)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["access_token"] == "" {
		t.Error("login: expected access_token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	srv := newTestServer(t)

	doRequest(t, srv, "POST", "/auth/register", `{"email":"carol@example.com","password":"correct-pw"}`)
	resp := doRequest(t, srv, "POST", "/auth/login", `{"email":"carol@example.com","password":"wrong-pw"}`)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: got %d, want 401", resp.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	srv := newTestServer(t)

	resp := doRequest(t, srv, "POST", "/auth/login", `{"email":"nobody@example.com","password":"pw"}`)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("unknown email: got %d, want 401", resp.Code)
	}
}

func TestSession_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/auth/session", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("session without token: got %d, want 401", rec.Code)
	}
}

func TestSession_WithValidToken(t *testing.T) {
	srv := newTestServer(t)

	token := registerAndGetToken(t, srv, "dave@example.com")
	resp := doRequestWithToken(t, srv, "GET", "/auth/session", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("session: got %d, want 200 — %s", resp.Code, resp.Body)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if out["email"] != "dave@example.com" {
		t.Errorf("session email: got %v, want dave@example.com", out["email"])
	}
}

func TestRefresh(t *testing.T) {
	srv := newTestServer(t)

	regResp := doRequest(t, srv, "POST", "/auth/register", `{"email":"eve@example.com","password":"pw12345678"}`)
	cookies := regResp.Result().Cookies()

	req := httptest.NewRequest("POST", "/auth/refresh", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: got %d, want 200 — %s", rec.Code, rec.Body)
	}
	var out map[string]string
	json.NewDecoder(rec.Body).Decode(&out)
	if out["access_token"] == "" {
		t.Error("refresh: expected new access_token")
	}
}

func TestLogout_ThenRefreshFails(t *testing.T) {
	srv := newTestServer(t)

	regResp := doRequest(t, srv, "POST", "/auth/register", `{"email":"frank@example.com","password":"pw12345678"}`)
	cookies := regResp.Result().Cookies()

	// logout
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: got %d, want 200", rec.Code)
	}

	// refresh with revoked token should fail
	req2 := httptest.NewRequest("POST", "/auth/refresh", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: got %d, want 401", rec2.Code)
	}
}
