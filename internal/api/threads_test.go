package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ybordag/cambium/internal/auth"
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

// fakeRhizomeThreadServerCapturing records the JSON body it receives so the
// test can assert on what Cambium actually forwarded to Rhizome.
func fakeRhizomeThreadServerCapturing(t *testing.T, captured *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"thread_id": "stub", "created": true})
	}))
}

func TestCreateThread_PassesThroughInitialContext(t *testing.T) {
	var captured map[string]any
	fake := fakeRhizomeThreadServerCapturing(t, &captured)
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "thread-context@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads",
		`{"initial_context":[{"subject_type":"plant","subject_id":"plant-uuid"}]}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("create thread with initial_context: got %d — %s", resp.Code, resp.Body)
	}

	ic, ok := captured["initial_context"].([]any)
	if !ok || len(ic) != 1 {
		t.Fatalf("initial_context not forwarded to rhizome: %v", captured)
	}
	entry := ic[0].(map[string]any)
	if entry["subject_type"] != "plant" || entry["subject_id"] != "plant-uuid" {
		t.Errorf("initial_context entry mismatch: %v", entry)
	}
}

func TestCreateThread_WithoutInitialContextOmitsField(t *testing.T) {
	var captured map[string]any
	fake := fakeRhizomeThreadServerCapturing(t, &captured)
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "thread-no-context@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads", `{"title":"No context"}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("create thread: got %d — %s", resp.Code, resp.Body)
	}
	if _, present := captured["initial_context"]; present {
		t.Errorf("expected initial_context to be omitted, got %v", captured["initial_context"])
	}
}

func TestCreateThread_RhizomeRejectsInitialContext_Returns400(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Entity not found or not accessible: plant/ghost"})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "thread-bad-context@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads",
		`{"initial_context":[{"subject_type":"plant","subject_id":"ghost"}]}`, token)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 propagated from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["error"] != "Entity not found or not accessible: plant/ghost" {
		t.Errorf("expected rhizome detail message propagated, got %v", out)
	}
}

func TestAddThreadContext_ProxiesToRhizome(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"thread_id":      "thread-1",
			"pinned_context": []map[string]string{{"subject_type": "plant", "subject_id": "p1"}},
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "context-add@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads/thread-1/context",
		`{"subject_type":"plant","subject_id":"p1"}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("add context: got %d — %s", resp.Code, resp.Body)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected rhizome to see POST, got %s", gotMethod)
	}
	if gotPath != "/internal/data/threads/thread-1/context" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
	if gotBody["subject_type"] != "plant" || gotBody["subject_id"] != "p1" {
		t.Errorf("body not forwarded: %v", gotBody)
	}
}

func TestRemoveThreadContext_ProxiesAsDelete(t *testing.T) {
	var gotPath, gotMethod string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"thread_id": "thread-1", "pinned_context": []any{}})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "context-remove@example.com")

	resp := doRequestWithToken(t, srv, "DELETE", "/api/v1/threads/thread-1/context/plant/p1", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("remove context: got %d — %s", resp.Code, resp.Body)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected rhizome to see DELETE (regression: must not collapse to POST), got %s", gotMethod)
	}
	if gotPath != "/internal/data/threads/thread-1/context/plant/p1" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
}

func TestGetThreadSessionContext_ProxiesToRhizome(t *testing.T) {
	var gotPath, gotMethod, gotQuery string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"time_text":   "45 minutes, all afternoon",
			"energy_text": "low, focused",
			"focus_text":  "How do I fertilize the cherry tomatoes?",
			"focus_context": []map[string]any{
				{"subject_type": "plant", "subject_id": "plant-1", "label": "Cherry Tomato (Sungold)"},
			},
			"source":     "user",
			"updated_at": "2026-06-25T16:44:56Z",
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-get@example.com")
	wantUserID, err := auth.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}

	resp := doRequestWithToken(t, srv, "GET", "/api/v1/threads/thread-1/session-context", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("get session context: got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["source"] != "user" || out["focus_text"] != "How do I fertilize the cherry tomatoes?" {
		t.Errorf("expected rhizome response forwarded verbatim, got %v", out)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("expected rhizome to see GET, got %s", gotMethod)
	}
	if gotPath != "/internal/data/threads/thread-1/session-context" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
	if gotQuery != "user_id="+wantUserID {
		t.Errorf("expected Cambium to inject user_id query, got %q", gotQuery)
	}
}

func TestPatchThreadSessionContext_ProxiesAsPatchWithBody(t *testing.T) {
	var gotPath, gotMethod, gotQuery string
	var gotBody map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"time_text":   nil,
			"energy_text": "low, focused",
			"focus_text":  "How do I fertilize the cherry tomatoes?",
			"focus_context": []map[string]any{
				{"subject_type": "plant", "subject_id": "plant-1", "label": "Cherry Tomato (Sungold)"},
				{"subject_type": "task", "subject_id": "task-1", "label": "Fertilize Tomato"},
			},
			"source":     "user",
			"updated_at": "2026-06-25T16:44:56Z",
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-patch@example.com")
	wantUserID, err := auth.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}

	resp := doRequestWithToken(t, srv, "PATCH", "/api/v1/threads/thread-1/session-context",
		`{"time_text":null,"energy_text":"low, focused","focus_text":"How do I fertilize the cherry tomatoes?","focus_context":[{"subject_type":"plant","subject_id":"plant-1"},{"subject_type":"task","subject_id":"task-1"}]}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("patch session context: got %d — %s", resp.Code, resp.Body)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("expected rhizome to see PATCH, got %s", gotMethod)
	}
	if gotPath != "/internal/data/threads/thread-1/session-context" {
		t.Errorf("unexpected rhizome path: %s", gotPath)
	}
	if gotQuery != "user_id="+wantUserID {
		t.Errorf("expected Cambium to inject user_id query, got %q", gotQuery)
	}
	if _, ok := gotBody["time_text"]; !ok || gotBody["time_text"] != nil {
		t.Errorf("explicit null time_text not forwarded: %v", gotBody)
	}
	if gotBody["energy_text"] != "low, focused" || gotBody["focus_text"] != "How do I fertilize the cherry tomatoes?" {
		t.Errorf("text fields not forwarded: %v", gotBody)
	}
	focusContext, ok := gotBody["focus_context"].([]any)
	if !ok || len(focusContext) != 2 {
		t.Errorf("focus_context entries not forwarded unchanged: %v", gotBody)
	}
}

func TestPatchThreadSessionContext_ForwardsEmptyFocusContextList(t *testing.T) {
	var gotBody map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"focus_context": []map[string]any{},
			"source":        "user",
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-empty-focus@example.com")

	resp := doRequestWithToken(t, srv, "PATCH", "/api/v1/threads/thread-1/session-context",
		`{"focus_context":[]}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("patch session context: got %d — %s", resp.Code, resp.Body)
	}
	focusContext, ok := gotBody["focus_context"].([]any)
	if !ok {
		t.Fatalf("focus_context was not forwarded as an array: %v", gotBody)
	}
	if len(focusContext) != 0 {
		t.Errorf("expected empty focus_context forwarded unchanged, got %v", gotBody)
	}
}

func TestPatchThreadSessionContext_ForwardsUnknownPayloadFieldsToRhizome(t *testing.T) {
	var gotBody map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"focus_text": "tomatoes",
			"source":     "user",
		})
	}))
	defer fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-unknown-fields@example.com")

	resp := doRequestWithToken(t, srv, "PATCH", "/api/v1/threads/thread-1/session-context",
		`{"focus_text":"tomatoes","client_only":"keep","focus_context":[{"subject_type":"plant","subject_id":"plant-1","label":"client label","client_meta":{"source":"verdant"}}]}`, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("patch session context: got %d — %s", resp.Code, resp.Body)
	}
	if gotBody["client_only"] != "keep" {
		t.Errorf("root unknown field was not forwarded unchanged: %v", gotBody)
	}
	focusContext, ok := gotBody["focus_context"].([]any)
	if !ok || len(focusContext) != 1 {
		t.Fatalf("focus_context entries not forwarded unchanged: %v", gotBody)
	}
	entry, ok := focusContext[0].(map[string]any)
	if !ok {
		t.Fatalf("focus_context entry has unexpected shape: %v", focusContext[0])
	}
	if entry["label"] != "client label" {
		t.Errorf("nested label field was not forwarded to Rhizome: %v", entry)
	}
	clientMeta, ok := entry["client_meta"].(map[string]any)
	if !ok || clientMeta["source"] != "verdant" {
		t.Errorf("nested unknown object was not forwarded unchanged: %v", entry)
	}
}

// fakeRhizomeStatus returns a server that always responds with the given
// status and JSON body, regardless of method or path — used to verify
// Cambium passes Rhizome's error responses through unchanged.
func fakeRhizomeStatus(t *testing.T, status int, jsonBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(jsonBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAddThreadContext_PassesThroughRhizome409OnDuplicate(t *testing.T) {
	fake := fakeRhizomeStatus(t, http.StatusConflict, `{"detail":"Entity already in thread context"}`)
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "context-409@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads/thread-1/context",
		`{"subject_type":"plant","subject_id":"p1"}`, token)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected 409 passed through from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["detail"] != "Entity already in thread context" {
		t.Errorf("expected rhizome body forwarded verbatim, got %v", out)
	}
}

func TestAddThreadContext_PassesThroughRhizome400OnLimit(t *testing.T) {
	fake := fakeRhizomeStatus(t, http.StatusBadRequest, `{"detail":"Thread context limit reached (max 10 items)"}`)
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "context-400@example.com")

	resp := doRequestWithToken(t, srv, "POST", "/api/v1/threads/thread-1/context",
		`{"subject_type":"bed","subject_id":"b11"}`, token)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 passed through from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["detail"] != "Thread context limit reached (max 10 items)" {
		t.Errorf("expected rhizome body forwarded verbatim, got %v", out)
	}
}

func TestRemoveThreadContext_PassesThroughRhizome404(t *testing.T) {
	fake := fakeRhizomeStatus(t, http.StatusNotFound, `{"detail":"Context entry not found"}`)
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "context-404@example.com")

	resp := doRequestWithToken(t, srv, "DELETE", "/api/v1/threads/thread-1/context/plant/ghost", "", token)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 passed through from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["detail"] != "Context entry not found" {
		t.Errorf("expected rhizome body forwarded verbatim, got %v", out)
	}
}

func TestGetThreadSessionContext_PassesThroughRhizome404(t *testing.T) {
	fake := fakeRhizomeStatus(t, http.StatusNotFound, `{"detail":"Thread not found"}`)
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-404@example.com")

	resp := doRequestWithToken(t, srv, "GET", "/api/v1/threads/thread-1/session-context", "", token)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 passed through from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["detail"] != "Thread not found" {
		t.Errorf("expected rhizome body forwarded verbatim, got %v", out)
	}
}

func TestPatchThreadSessionContext_PassesThroughRhizome400(t *testing.T) {
	fake := fakeRhizomeStatus(t, http.StatusBadRequest, `{"detail":"empty session context patch"}`)
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-400@example.com")

	resp := doRequestWithToken(t, srv, "PATCH", "/api/v1/threads/thread-1/session-context", `{}`, token)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 passed through from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["detail"] != "empty session context patch" {
		t.Errorf("expected rhizome body forwarded verbatim, got %v", out)
	}
}

func TestPatchThreadSessionContext_PassesThroughRhizome422(t *testing.T) {
	fake := fakeRhizomeStatus(t, http.StatusUnprocessableEntity, `{"detail":[{"loc":["body","focus_context",0,"label"],"msg":"extra fields not permitted"}]}`)
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-422@example.com")

	resp := doRequestWithToken(t, srv, "PATCH", "/api/v1/threads/thread-1/session-context",
		`{"focus_context":[{"subject_type":"plant","subject_id":"p1","label":"client label"}]}`, token)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 passed through from rhizome, got %d — %s", resp.Code, resp.Body)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if _, ok := out["detail"].([]any); !ok {
		t.Errorf("expected rhizome validation body forwarded verbatim, got %v", out)
	}
}

func TestThreadSessionContext_RhizomeUnavailableReturns502(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	fake.Close()
	t.Setenv("RHIZOME_INTERNAL_URL", fake.URL)

	srv := newTestServer(t)
	token := registerAndGetToken(t, srv, "session-context-502@example.com")

	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: "GET"},
		{name: "patch", method: "PATCH", body: `{"focus_text":"tomatoes"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doRequestWithToken(t, srv, tt.method, "/api/v1/threads/thread-1/session-context", tt.body, token)
			if resp.Code != http.StatusBadGateway {
				t.Fatalf("expected 502 when rhizome is unavailable, got %d — %s", resp.Code, resp.Body)
			}
			var out map[string]string
			json.NewDecoder(resp.Body).Decode(&out)
			if out["error"] == "" {
				t.Errorf("expected structured error body, got %v", out)
			}
		})
	}
}

func TestThreadContext_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	resp := doRequest(t, srv, "POST", "/api/v1/threads/thread-1/context", `{"subject_type":"plant","subject_id":"p1"}`)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("add context without auth: got %d, want 401", resp.Code)
	}

	resp = doRequest(t, srv, "DELETE", "/api/v1/threads/thread-1/context/plant/p1", "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("remove context without auth: got %d, want 401", resp.Code)
	}

	resp = doRequest(t, srv, "GET", "/api/v1/threads/thread-1/session-context", "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("get session context without auth: got %d, want 401", resp.Code)
	}

	resp = doRequest(t, srv, "PATCH", "/api/v1/threads/thread-1/session-context", `{"focus_text":"tomatoes"}`)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("patch session context without auth: got %d, want 401", resp.Code)
	}
}
