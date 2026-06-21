package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ybordag/cambium/internal/auth"
)

func TestBatchRemovePlants_ProxiesToRhizome(t *testing.T) {
	var gotPath string
	var gotUserID string
	var gotBody map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUserID = r.URL.Query().Get("user_id")
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if gotUserID == "" {
			t.Errorf("user_id not forwarded: %s", r.URL.RawQuery)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "plant-1", "name": "Basil", "status": "removed"},
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "batch-remove-user@example.com")
	wantUserID, err := auth.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}

	resp := doRequestWithToken(
		t,
		srv,
		"PATCH",
		"/api/v1/garden/plants/batch/remove",
		`{"name":"Basil","reason":"thinning"}`,
		token,
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("batch remove: got %d - %s", resp.Code, resp.Body)
	}
	if gotPath != "/internal/data/garden/plants/batch/remove" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
	if gotUserID != wantUserID {
		t.Errorf("expected authenticated user_id %q, got %q", wantUserID, gotUserID)
	}
	if gotBody["name"] != "Basil" || gotBody["reason"] != "thinning" {
		t.Errorf("unexpected forwarded body: %v", gotBody)
	}
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out) != 1 || out[0]["status"] != "removed" {
		t.Fatalf("expected rhizome response forwarded verbatim, got %v", out)
	}
}
