package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeRhizomeData spins up a test server standing in for Rhizome's
// /internal/data/* surface, recording the method, path, and body it saw.
func fakeRhizomeData(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("RHIZOME_INTERNAL_URL", srv.URL)
}

func newProxyRequest(method, path, body string, userID string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	return withUserID(r, userID)
}

// This file intentionally guards against the regression of the
// method-collapsing bug fixed in dispatchData: every non-GET, non-DELETE
// method must reach Rhizome under its own HTTP verb, not silently converted
// to POST.

func TestDispatchData_GETForwardsQueryParams(t *testing.T) {
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("status") != "pending" {
			t.Errorf("query params not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodGet, "/api/v1/tasks?status=pending", "", "user-1")
	ph.proxyData("tasks")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
}

func TestDispatchData_PATCHForwardsAsPatchNotPost(t *testing.T) {
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH to reach rhizome unchanged, got %s", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "resolved" {
			t.Errorf("body not forwarded: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodPatch, "/api/v1/incidents/abc", `{"status":"resolved"}`, "user-1")
	ph.proxyDataWithPathParam("incidents", "id")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
}

func TestDispatchData_DELETESendsDeleteNoBody(t *testing.T) {
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE to reach rhizome unchanged, got %s", r.Method)
		}
		if r.URL.Query().Get("user_id") != "user-7" {
			t.Errorf("user_id not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"deleted"}`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodDelete, "/api/v1/threads/thread-1", "", "user-7")
	ph.proxyDataWithPathParam("threads", "id")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
}

func TestDispatchData_MultiSegmentPathForwardedVerbatim(t *testing.T) {
	var gotPath string
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"thread_id":"thread-1","pinned_context":[]}`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodDelete, "/api/v1/threads/thread-1/context/plant/plant-9", "", "user-1")
	ph.proxyDataWithPathParam("threads", "id")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
	want := "/internal/data/threads/thread-1/context/plant/plant-9"
	if gotPath != want {
		t.Errorf("got path %q, want %q", gotPath, want)
	}
}

func TestDispatchData_POSTContextForwardsBody(t *testing.T) {
	var gotBody map[string]any
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"thread_id":"thread-1","pinned_context":[{"subject_type":"plant","subject_id":"p1"}]}`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodPost, "/api/v1/threads/thread-1/context", `{"subject_type":"plant","subject_id":"p1"}`, "user-1")
	ph.proxyDataWithPathParam("threads", "id")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
	if gotBody["subject_type"] != "plant" || gotBody["subject_id"] != "p1" {
		t.Errorf("body not forwarded correctly: %v", gotBody)
	}
}

func TestDispatchData_SearchForwardsQueryParams(t *testing.T) {
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/data/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "tomato" {
			t.Errorf("q param not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[],"by_type":{}}`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodGet, "/api/v1/search?q=tomato", "", "user-1")
	ph.proxyData("search")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
}

func TestNotificationStream_ForwardsAsSSE(t *testing.T) {
	ssePayload := "data: {\"type\":\"heartbeat\"}\n\n"
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/internal/data/notifications/stream" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("user_id") != "user-1" {
			t.Errorf("user_id not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(ssePayload))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodGet, "/api/v1/notifications/stream", "", "user-1")
	ph.notificationStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("got Content-Type %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != ssePayload {
		t.Errorf("got body %q, want %q", rec.Body.String(), ssePayload)
	}
}

func TestNotificationStream_RhizomeUnavailableReturns502(t *testing.T) {
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodGet, "/api/v1/notifications/stream", "", "user-1")
	ph.notificationStream(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", rec.Code)
	}
}

func TestNotifications_DataProxyForwardsSinceParam(t *testing.T) {
	fakeRhizomeData(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/data/notifications" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("since") != "2026-06-20T00:00:00" {
			t.Errorf("since param not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"alerts":[],"pending_interactions":[],"active_jobs":[]}`))
	})

	ph := newProxyHandler(nil)
	rec := httptest.NewRecorder()
	req := newProxyRequest(http.MethodGet, "/api/v1/notifications?since=2026-06-20T00:00:00", "", "user-1")
	ph.proxyData("notifications")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body)
	}
}
