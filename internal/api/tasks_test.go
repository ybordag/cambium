package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTasksBlocked_ProxiesToRhizomeWithProjectFilter(t *testing.T) {
	var gotPath string
	var gotQuery string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		if r.URL.Query().Get("project_id") != "project-1" {
			t.Errorf("project_id not forwarded: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("user_id") == "" {
			t.Errorf("user_id not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "task-1", "title": "Blocked task", "blocked": true},
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "tasks-blocked-user@example.com")

	resp := doRequestWithToken(t, srv, "GET", "/api/v1/tasks/blocked?project_id=project-1", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("tasks blocked: got %d - %s", resp.Code, resp.Body)
	}
	if gotPath != "/internal/data/tasks/blocked" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
	if gotQuery == "" {
		t.Errorf("expected forwarded query params")
	}
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out) != 1 || out[0]["id"] != "task-1" {
		t.Fatalf("expected rhizome response forwarded verbatim, got %v", out)
	}
}
