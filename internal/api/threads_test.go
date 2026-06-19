package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeRhizomeThreadServer returns a test HTTP server that mimics Rhizome's
// POST /internal/data/threads endpoint, always reporting created=true.
func fakeRhizomeThreadServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"thread_id": "stub", "created": true})
	}))
}

func TestCreateThread_ReturnsThreadID(t *testing.T) {
	fake := fakeRhizomeThreadServer(t)
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "thread-user@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("create thread: got %d — %s", resp.Code, resp.Body)
	}

	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)

	threadID := out["thread_id"]
	if threadID == "" {
		t.Fatal("expected thread_id in response, got empty")
	}

	// Must be three hyphen-separated words
	parts := strings.Split(threadID, "-")
	if len(parts) != 3 {
		t.Errorf("thread_id %q should have 3 words, got %d", threadID, len(parts))
	}
}

func TestCreateThread_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	resp := doRequest(t, srv, "POST", "/api/v1/threads", "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("create thread without auth: got %d, want 401", resp.Code)
	}
}

func TestCreateThread_WithOptionalTitle(t *testing.T) {
	fake := fakeRhizomeThreadServer(t)
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "thread-title@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads",
		`{"title":"Spring planting"}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("create thread with title: got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["thread_id"] == "" {
		t.Error("expected thread_id in response")
	}
}
