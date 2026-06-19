package api_test

import (
	"net/http"
	"testing"
)

// TestAllProtectedRoutesReject401 is a security sweep verifying that every
// route under /api/v1 rejects unauthenticated requests with 401.
// If a new route is added without RequireAuth this test will catch it.
func TestAllProtectedRoutesReject401(t *testing.T) {
	srv := newTestServer(t)

	routes := []struct {
		method string
		path   string
	}{
		// Auth (protected)
		{"GET", "/auth/session"},

		// Keys
		{"PUT", "/api/v1/auth/keys"},
		{"GET", "/api/v1/auth/keys"},
		{"DELETE", "/api/v1/auth/keys/gemini"},

		// Chat
		{"POST", "/api/v1/chat"},
		{"POST", "/api/v1/chat/stream"},
		{"POST", "/api/v1/chat/resume"},
		{"POST", "/api/v1/chat/resume/stream"},

		// Threads
		{"POST", "/api/v1/threads"},
		{"GET", "/api/v1/threads"},
		{"GET", "/api/v1/threads/some-thread-id"},
		{"GET", "/api/v1/threads/some-thread-id/messages"},
		{"DELETE", "/api/v1/threads/some-thread-id"},

		// Garden
		{"GET", "/api/v1/garden/profile"},
		{"PATCH", "/api/v1/garden/profile"},
		{"GET", "/api/v1/garden/beds"},
		{"GET", "/api/v1/garden/containers"},
		{"POST", "/api/v1/garden/containers"},
		{"GET", "/api/v1/garden/plants"},
		{"POST", "/api/v1/garden/plants"},
		{"GET", "/api/v1/garden/batches"},
		{"GET", "/api/v1/garden/search"},

		// Tasks
		{"GET", "/api/v1/tasks/daily"},
		{"GET", "/api/v1/tasks/due"},
		{"GET", "/api/v1/tasks/blocked"},
		{"GET", "/api/v1/tasks/some-task-id"},
		{"POST", "/api/v1/tasks/some-task-id/start"},
		{"POST", "/api/v1/tasks/some-task-id/complete"},

		// Projects
		{"GET", "/api/v1/projects"},
		{"POST", "/api/v1/projects"},
		{"GET", "/api/v1/projects/some-project-id"},

		// Triage
		{"POST", "/api/v1/triage/run"},
		{"GET", "/api/v1/triage/latest"},

		// Weather
		{"GET", "/api/v1/weather/latest"},
		{"POST", "/api/v1/weather/refresh"},
		{"POST", "/api/v1/weather/tasks/draft"},

		// Incidents
		{"GET", "/api/v1/incidents"},
		{"POST", "/api/v1/incidents"},

		// Interactions
		{"GET", "/api/v1/interactions/pending"},
		{"GET", "/api/v1/interactions/recent"},

		// Alerts
		{"GET", "/api/v1/alerts"},

		// Monitor
		{"GET", "/api/v1/monitor/runs"},

		// Activity
		{"GET", "/api/v1/activity"},
	}

	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			resp := doRequest(t, srv, r.method, r.path, "")
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("%s %s without token: got %d, want 401",
					r.method, r.path, resp.Code)
			}
		})
	}
}

// TestPublicRoutesAllowUnauthenticated verifies that auth and health endpoints
// do not require a token.
func TestPublicRoutesAllowUnauthenticated(t *testing.T) {
	srv := newTestServer(t)

	// These should never return 401 (no JWT required).
	// Note: /auth/refresh is public but returns 401 when called without a
	// refresh cookie — that is correct auth behaviour, not a middleware rejection.
	publicRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"POST", "/auth/register"},
		{"POST", "/auth/login"},
		{"POST", "/auth/logout"},
	}

	for _, r := range publicRoutes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			resp := doRequest(t, srv, r.method, r.path, "")
			if resp.Code == http.StatusUnauthorized {
				t.Errorf("%s %s should be public but returned 401", r.method, r.path)
			}
		})
	}
}

// TestSwaggerEndpointServes verifies the Swagger UI endpoint is accessible.
func TestSwaggerEndpointServes(t *testing.T) {
	srv := newTestServer(t)
	resp := doRequest(t, srv, "GET", "/docs/", "")
	// Swagger UI responds with 301 redirect to /docs/index.html
	if resp.Code != http.StatusMovedPermanently && resp.Code != http.StatusOK {
		t.Errorf("GET /docs/: got %d, want 200 or 301", resp.Code)
	}
}

// TestThreadProxyRoutesRequireAuth verifies all thread list/get/delete routes
// are protected — only POST (create) was previously tested.
func TestThreadProxyRoutesRequireAuth(t *testing.T) {
	srv := newTestServer(t)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/threads"},
		{"GET", "/api/v1/threads/silver-fern-cascade"},
		{"GET", "/api/v1/threads/silver-fern-cascade/messages"},
		{"DELETE", "/api/v1/threads/silver-fern-cascade"},
	}

	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			resp := doRequest(t, srv, r.method, r.path, "")
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: got %d, want 401", r.method, r.path, resp.Code)
			}
		})
	}
}

// TestChatEndpointsRequireAuth verifies all chat endpoints reject unauthenticated requests.
func TestChatEndpointsRequireAuth(t *testing.T) {
	srv := newTestServer(t)

	endpoints := []string{
		"/api/v1/chat",
		"/api/v1/chat/stream",
		"/api/v1/chat/resume",
		"/api/v1/chat/resume/stream",
	}

	for _, path := range endpoints {
		t.Run(path, func(t *testing.T) {
			resp := doRequest(t, srv, "POST", path, `{"message":"test"}`)
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("POST %s: got %d, want 401", path, resp.Code)
			}
		})
	}
}
