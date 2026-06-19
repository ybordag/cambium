package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires all routes and returns the root handler.
func NewRouter(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	ph := newProxyHandler(pool)

	ah := &authHandler{pool: pool}
	kh := &keysHandler{pool: pool}

	// Public
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /auth/register", ah.register)
	mux.HandleFunc("POST /auth/login", ah.login)
	mux.HandleFunc("POST /auth/refresh", ah.refresh)
	mux.HandleFunc("POST /auth/logout", ah.logout)

	// Protected — require valid JWT
	mux.Handle("GET /auth/session", RequireAuth(http.HandlerFunc(ah.session)))
	mux.Handle("PUT /api/v1/auth/keys", RequireAuth(http.HandlerFunc(kh.set)))
	mux.Handle("GET /api/v1/auth/keys", RequireAuth(http.HandlerFunc(kh.list)))
	mux.Handle("DELETE /api/v1/auth/keys/{provider}", RequireAuth(http.HandlerFunc(kh.delete)))

	// Chat — agent proxy
	mux.Handle("POST /api/v1/chat", RequireAuth(http.HandlerFunc(ph.chat)))
	mux.Handle("POST /api/v1/chat/stream", RequireAuth(http.HandlerFunc(ph.chatStream)))
	mux.Handle("POST /api/v1/chat/resume", RequireAuth(http.HandlerFunc(ph.chatResume)))
	mux.Handle("POST /api/v1/chat/resume/stream", RequireAuth(http.HandlerFunc(ph.chatResumeStream)))

	// Alerts
	mux.Handle("GET /api/v1/alerts", RequireAuth(http.HandlerFunc(ph.proxyData("alerts"))))
	mux.Handle("POST /api/v1/alerts/{id}/dismiss", RequireAuth(ph.proxyDataWithPathParam("alerts", "id")))

	// Tasks
	mux.Handle("GET /api/v1/tasks", RequireAuth(http.HandlerFunc(ph.proxyData("tasks"))))
	mux.Handle("GET /api/v1/tasks/daily", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/daily"))))
	mux.Handle("GET /api/v1/tasks/{id}", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/complete", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/skip", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/defer", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))

	// Projects
	mux.Handle("GET /api/v1/projects", RequireAuth(http.HandlerFunc(ph.proxyData("projects"))))
	mux.Handle("GET /api/v1/projects/{id}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/progress", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/tasks", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))

	// Monitor
	mux.Handle("GET /api/v1/monitor/runs", RequireAuth(http.HandlerFunc(ph.proxyData("monitor/runs"))))
	mux.Handle("GET /api/v1/monitor/runs/{id}", RequireAuth(ph.proxyDataWithPathParam("monitor/runs", "id")))

	return mux
}
