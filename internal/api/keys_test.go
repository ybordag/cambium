package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestKeys_SetAndList(t *testing.T) {
	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "keys1@example.com")

	resp := doRequestWithToken(t, srv, "PUT", "/api/v1/auth/keys",
		`{"provider":"gemini","key":"AIzaFakeKey123"}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("set key: got %d — %s", resp.Code, resp.Body)
	}

	resp = doRequestWithToken(t, srv, "GET", "/api/v1/auth/keys", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("list keys: got %d", resp.Code)
	}
	var configured map[string]bool
	json.NewDecoder(resp.Body).Decode(&configured)
	if !configured["gemini"] {
		t.Error("expected gemini to be configured")
	}
	if configured["openai"] || configured["anthropic"] {
		t.Error("openai and anthropic should not be configured")
	}
}

func TestKeys_Delete(t *testing.T) {
	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "keys2@example.com")

	doRequestWithToken(t, srv, "PUT", "/api/v1/auth/keys",
		`{"provider":"openai","key":"sk-fake"}`, token)

	resp := doRequestWithToken(t, srv, "DELETE", "/api/v1/auth/keys/openai", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("delete key: got %d — %s", resp.Code, resp.Body)
	}

	resp = doRequestWithToken(t, srv, "GET", "/api/v1/auth/keys", "", token)
	var configured map[string]bool
	json.NewDecoder(resp.Body).Decode(&configured)
	if configured["openai"] {
		t.Error("openai should not be configured after delete")
	}
}

func TestKeys_RequireAuth(t *testing.T) {
	srv := newTestServer(t)

	resp := doRequest(t, srv, "GET", "/api/v1/auth/keys", "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("list keys without token: got %d, want 401", resp.Code)
	}
}

func TestKeys_InvalidProvider(t *testing.T) {
	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "keys3@example.com")

	resp := doRequestWithToken(t, srv, "PUT", "/api/v1/auth/keys",
		`{"provider":"fakeprovider","key":"somekey"}`, token)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("invalid provider: got %d, want 400", resp.Code)
	}
}
