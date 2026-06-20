package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnifiedSearch_ProxiesToRhizome(t *testing.T) {
	var gotPath string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Query().Get("q") != "tomato" {
			t.Errorf("q param not forwarded: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("types") != "plant,task" {
			t.Errorf("types param not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"by_type": map[string]int{"plant": 0, "task": 0},
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "search-user@example.com")

	resp := doRequestWithToken(t, srv, "GET", "/api/v1/search?q=tomato&types=plant,task", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("search: got %d — %s", resp.Code, resp.Body)
	}
	if gotPath != "/internal/data/search" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
}

func TestUnifiedSearch_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	resp := doRequest(t, srv, "GET", "/api/v1/search?q=tomato", "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("search without auth: got %d, want 401", resp.Code)
	}
}
