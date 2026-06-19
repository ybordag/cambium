package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	httpSwagger "github.com/swaggo/http-swagger"
)

// NewRouter wires all routes and returns the root handler.
func NewRouter(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	ph := newProxyHandler(pool)
	ah := &authHandler{pool: pool}
	kh := &keysHandler{pool: pool}
	th := &threadHandler{pool: pool, rhizome: ph.rhizome}

	// -------------------------------------------------------------------------
	// Public
	// -------------------------------------------------------------------------

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /auth/register", ah.register)
	mux.HandleFunc("POST /auth/login", ah.login)
	mux.HandleFunc("POST /auth/refresh", ah.refresh)
	mux.HandleFunc("POST /auth/logout", ah.logout)

	// -------------------------------------------------------------------------
	// Auth + key management (require JWT)
	// -------------------------------------------------------------------------

	mux.Handle("GET /auth/session", RequireAuth(http.HandlerFunc(ah.session)))
	mux.Handle("PATCH /auth/profile", RequireAuth(http.HandlerFunc(ah.profile)))
	mux.Handle("POST /auth/password", RequireAuth(http.HandlerFunc(ah.password)))
	mux.Handle("PUT /api/v1/auth/keys", RequireAuth(http.HandlerFunc(kh.set)))
	mux.Handle("GET /api/v1/auth/keys", RequireAuth(http.HandlerFunc(kh.list)))
	mux.Handle("DELETE /api/v1/auth/keys/{provider}", RequireAuth(http.HandlerFunc(kh.delete)))

	// -------------------------------------------------------------------------
	// Chat — agent proxy (streaming + non-streaming)
	// -------------------------------------------------------------------------

	mux.Handle("POST /api/v1/chat", RequireAuth(http.HandlerFunc(ph.chat)))
	mux.Handle("POST /api/v1/chat/stream", RequireAuth(http.HandlerFunc(ph.chatStream)))
	mux.Handle("POST /api/v1/chat/resume", RequireAuth(http.HandlerFunc(ph.chatResume)))
	mux.Handle("POST /api/v1/chat/resume/stream", RequireAuth(http.HandlerFunc(ph.chatResumeStream)))

	// -------------------------------------------------------------------------
	// Garden — profile
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/garden/profile", RequireAuth(http.HandlerFunc(ph.proxyData("garden/profile"))))
	mux.Handle("PATCH /api/v1/garden/profile", RequireAuth(http.HandlerFunc(ph.proxyData("garden/profile"))))

	// -------------------------------------------------------------------------
	// Garden — beds
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/garden/beds", RequireAuth(http.HandlerFunc(ph.proxyData("garden/beds"))))
	mux.Handle("POST /api/v1/garden/beds", RequireAuth(http.HandlerFunc(ph.proxyData("garden/beds"))))
	mux.Handle("GET /api/v1/garden/beds/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))
	mux.Handle("PATCH /api/v1/garden/beds/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))
	mux.Handle("DELETE /api/v1/garden/beds/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))
	mux.Handle("GET /api/v1/garden/beds/{id}/care/state", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))
	mux.Handle("GET /api/v1/garden/beds/{id}/care/history", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))
	mux.Handle("POST /api/v1/garden/beds/{id}/care", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))
	mux.Handle("GET /api/v1/garden/beds/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("garden/beds", "id")))

	// -------------------------------------------------------------------------
	// Garden — containers
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/garden/containers", RequireAuth(http.HandlerFunc(ph.proxyData("garden/containers"))))
	mux.Handle("POST /api/v1/garden/containers", RequireAuth(http.HandlerFunc(ph.proxyData("garden/containers"))))
	mux.Handle("GET /api/v1/garden/containers/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))
	mux.Handle("PATCH /api/v1/garden/containers/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))
	mux.Handle("DELETE /api/v1/garden/containers/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))
	mux.Handle("GET /api/v1/garden/containers/{id}/care/state", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))
	mux.Handle("GET /api/v1/garden/containers/{id}/care/history", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))
	mux.Handle("POST /api/v1/garden/containers/{id}/care", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))
	mux.Handle("GET /api/v1/garden/containers/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("garden/containers", "id")))

	// -------------------------------------------------------------------------
	// Garden — plants (batch routes must precede {id} wildcard)
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/garden/plants", RequireAuth(http.HandlerFunc(ph.proxyData("garden/plants"))))
	mux.Handle("POST /api/v1/garden/plants", RequireAuth(http.HandlerFunc(ph.proxyData("garden/plants"))))
	mux.Handle("POST /api/v1/garden/plants/batch", RequireAuth(http.HandlerFunc(ph.proxyData("garden/plants/batch"))))
	mux.Handle("PATCH /api/v1/garden/plants/batch/remove", RequireAuth(http.HandlerFunc(ph.proxyData("garden/plants/batch/remove"))))
	mux.Handle("PATCH /api/v1/garden/plants/batch", RequireAuth(http.HandlerFunc(ph.proxyData("garden/plants/batch"))))
	mux.Handle("GET /api/v1/garden/plants/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("PATCH /api/v1/garden/plants/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("PATCH /api/v1/garden/plants/{id}/remove", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("DELETE /api/v1/garden/plants/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("GET /api/v1/garden/plants/{id}/care/state", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("GET /api/v1/garden/plants/{id}/care/history", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("POST /api/v1/garden/plants/{id}/care", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))
	mux.Handle("GET /api/v1/garden/plants/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("garden/plants", "id")))

	// -------------------------------------------------------------------------
	// Garden — batches
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/garden/batches", RequireAuth(http.HandlerFunc(ph.proxyData("garden/batches"))))
	mux.Handle("DELETE /api/v1/garden/batches/{id}", RequireAuth(ph.proxyDataWithPathParam("garden/batches", "id")))
	mux.Handle("GET /api/v1/garden/batches/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("garden/batches", "id")))

	// -------------------------------------------------------------------------
	// Garden — search
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/garden/search", RequireAuth(http.HandlerFunc(ph.proxyData("garden/search"))))
	mux.Handle("GET /api/v1/garden/locations/{location}", RequireAuth(ph.proxyDataWithPathParam("garden/locations", "location")))

	// -------------------------------------------------------------------------
	// Projects
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/projects", RequireAuth(http.HandlerFunc(ph.proxyData("projects"))))
	mux.Handle("POST /api/v1/projects", RequireAuth(http.HandlerFunc(ph.proxyData("projects"))))
	mux.Handle("GET /api/v1/projects/{id}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("PATCH /api/v1/projects/{id}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("DELETE /api/v1/projects/{id}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/progress", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/brief", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("PATCH /api/v1/projects/{id}/brief", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/proposals", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/proposals/{proposalId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/proposals/{proposalId}/accept", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/tasks", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("PATCH /api/v1/projects/{id}/tasks/bulk", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/tasks/generate", RequireAuth(http.HandlerFunc(ph.triggerTaskGeneration)))
	mux.Handle("GET /api/v1/projects/{id}/series", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/beds", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/beds/batch", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/beds/{bedId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("DELETE /api/v1/projects/{id}/beds/{bedId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/containers", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/containers/batch", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/containers/{containerId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("DELETE /api/v1/projects/{id}/containers/{containerId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/plants/{plantId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("DELETE /api/v1/projects/{id}/plants/{plantId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/expenses", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("POST /api/v1/projects/{id}/expenses", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/expenses/summary", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("PATCH /api/v1/projects/{id}/expenses/{expenseId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("DELETE /api/v1/projects/{id}/expenses/{expenseId}", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))
	mux.Handle("GET /api/v1/projects/{id}/shopping", RequireAuth(ph.proxyDataWithPathParam("projects", "id")))

	// -------------------------------------------------------------------------
	// Tasks (literal routes before {id} wildcard)
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/tasks", RequireAuth(http.HandlerFunc(ph.proxyData("tasks"))))
	mux.Handle("POST /api/v1/tasks", RequireAuth(http.HandlerFunc(ph.proxyData("tasks"))))
	mux.Handle("GET /api/v1/tasks/daily", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/daily"))))
	mux.Handle("GET /api/v1/tasks/due", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/due"))))
	mux.Handle("GET /api/v1/tasks/blocked", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/blocked"))))
	mux.Handle("POST /api/v1/tasks/materialize", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/materialize"))))
	mux.Handle("POST /api/v1/tasks/series", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/series"))))
	mux.Handle("PATCH /api/v1/tasks/series/{id}", RequireAuth(ph.proxyDataWithPathParam("tasks/series", "id")))
	mux.Handle("DELETE /api/v1/tasks/series/{id}", RequireAuth(ph.proxyDataWithPathParam("tasks/series", "id")))
	mux.Handle("GET /api/v1/tasks/{id}", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("PATCH /api/v1/tasks/{id}", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("DELETE /api/v1/tasks/{id}", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/start", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/complete", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/skip", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/defer", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("POST /api/v1/tasks/{id}/dependencies", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("DELETE /api/v1/tasks/{id}/dependencies/{blockingId}", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("GET /api/v1/tasks/{id}/blockers", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))
	mux.Handle("GET /api/v1/tasks/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("tasks", "id")))

	// -------------------------------------------------------------------------
	// Triage
	// -------------------------------------------------------------------------

	mux.Handle("POST /api/v1/triage/run", RequireAuth(http.HandlerFunc(ph.triggerTriage)))
	mux.Handle("GET /api/v1/triage/latest", RequireAuth(http.HandlerFunc(ph.proxyData("triage/latest"))))
	mux.Handle("GET /api/v1/triage/recommendations", RequireAuth(http.HandlerFunc(ph.proxyData("triage/recommendations"))))
	mux.Handle("POST /api/v1/triage/monitor", RequireAuth(http.HandlerFunc(ph.proxyData("triage/monitor"))))

	// -------------------------------------------------------------------------
	// Weather
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/weather/latest", RequireAuth(http.HandlerFunc(ph.proxyData("weather/latest"))))
	mux.Handle("POST /api/v1/weather/refresh", RequireAuth(http.HandlerFunc(ph.proxyData("weather/refresh"))))
	mux.Handle("GET /api/v1/weather/tasks/impacted", RequireAuth(http.HandlerFunc(ph.proxyData("weather/tasks/impacted"))))
	mux.Handle("POST /api/v1/weather/tasks/draft", RequireAuth(http.HandlerFunc(ph.triggerWeatherDraft)))
	mux.Handle("PATCH /api/v1/weather/changesets/{id}/approve", RequireAuth(ph.proxyDataWithPathParam("weather/changesets", "id")))
	mux.Handle("POST /api/v1/weather/monitor", RequireAuth(http.HandlerFunc(ph.proxyData("weather/monitor"))))

	// -------------------------------------------------------------------------
	// Incidents + treatment plans
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/incidents", RequireAuth(http.HandlerFunc(ph.proxyData("incidents"))))
	mux.Handle("POST /api/v1/incidents", RequireAuth(http.HandlerFunc(ph.proxyData("incidents"))))
	mux.Handle("GET /api/v1/incidents/{id}", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("PATCH /api/v1/incidents/{id}", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("DELETE /api/v1/incidents/{id}", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("PATCH /api/v1/incidents/{id}/resolve", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("POST /api/v1/incidents/{id}/treatment", RequireAuth(http.HandlerFunc(ph.triggerTreatmentDraft)))
	mux.Handle("POST /api/v1/incidents/{id}/treatment/manual", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("GET /api/v1/incidents/{id}/treatment", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("GET /api/v1/incidents/{id}/activity", RequireAuth(ph.proxyDataWithPathParam("incidents", "id")))
	mux.Handle("PATCH /api/v1/treatment-plans/{id}", RequireAuth(ph.proxyDataWithPathParam("treatment-plans", "id")))
	mux.Handle("DELETE /api/v1/treatment-plans/{id}", RequireAuth(ph.proxyDataWithPathParam("treatment-plans", "id")))
	mux.Handle("PATCH /api/v1/treatment-plans/{id}/approve", RequireAuth(ph.proxyDataWithPathParam("treatment-plans", "id")))

	// -------------------------------------------------------------------------
	// Interactions
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/interactions/pending", RequireAuth(http.HandlerFunc(ph.proxyData("interactions/pending"))))
	mux.Handle("GET /api/v1/interactions/recent", RequireAuth(http.HandlerFunc(ph.proxyData("interactions/recent"))))
	mux.Handle("GET /api/v1/interactions/{id}", RequireAuth(ph.proxyDataWithPathParam("interactions", "id")))
	mux.Handle("POST /api/v1/interactions/{id}/resolve", RequireAuth(ph.proxyDataWithPathParam("interactions", "id")))

	// -------------------------------------------------------------------------
	// Alerts + monitor runs
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/alerts", RequireAuth(http.HandlerFunc(ph.proxyData("alerts"))))
	mux.Handle("POST /api/v1/alerts/{id}/dismiss", RequireAuth(ph.proxyDataWithPathParam("alerts", "id")))
	mux.Handle("GET /api/v1/monitor/runs", RequireAuth(http.HandlerFunc(ph.proxyData("monitor/runs"))))
	mux.Handle("GET /api/v1/monitor/runs/{id}", RequireAuth(ph.proxyDataWithPathParam("monitor/runs", "id")))
	mux.Handle("POST /api/v1/tasks/series/run", RequireAuth(http.HandlerFunc(ph.proxyData("tasks/series/run"))))

	// -------------------------------------------------------------------------
	// Threads — conversation management
	// -------------------------------------------------------------------------

	mux.Handle("POST /api/v1/threads", RequireAuth(http.HandlerFunc(th.createThread)))
	mux.Handle("GET /api/v1/threads", RequireAuth(http.HandlerFunc(ph.proxyData("threads"))))
	mux.Handle("GET /api/v1/threads/{id}/messages", RequireAuth(ph.proxyDataWithPathParam("threads", "id")))
	mux.Handle("GET /api/v1/threads/{id}", RequireAuth(ph.proxyDataWithPathParam("threads", "id")))
	mux.Handle("DELETE /api/v1/threads/{id}", RequireAuth(ph.proxyDataWithPathParam("threads", "id")))

	// -------------------------------------------------------------------------
	// Calendar
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/calendar/annotations", RequireAuth(http.HandlerFunc(ph.proxyData("calendar/annotations"))))
	mux.Handle("POST /api/v1/calendar/annotations", RequireAuth(http.HandlerFunc(ph.proxyData("calendar/annotations"))))
	mux.Handle("PATCH /api/v1/calendar/annotations/{id}", RequireAuth(ph.proxyDataWithPathParam("calendar/annotations", "id")))
	mux.Handle("DELETE /api/v1/calendar/annotations/{id}", RequireAuth(ph.proxyDataWithPathParam("calendar/annotations", "id")))

	// -------------------------------------------------------------------------
	// Shopping list
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/shopping", RequireAuth(http.HandlerFunc(ph.proxyData("shopping"))))
	mux.Handle("POST /api/v1/shopping", RequireAuth(http.HandlerFunc(ph.proxyData("shopping"))))
	mux.Handle("PATCH /api/v1/shopping/{id}", RequireAuth(ph.proxyDataWithPathParam("shopping", "id")))
	mux.Handle("DELETE /api/v1/shopping/{id}", RequireAuth(ph.proxyDataWithPathParam("shopping", "id")))
	mux.Handle("POST /api/v1/shopping/{id}/purchase", RequireAuth(ph.proxyDataWithPathParam("shopping", "id")))

	// -------------------------------------------------------------------------
	// Activity
	// -------------------------------------------------------------------------

	mux.Handle("GET /api/v1/activity", RequireAuth(http.HandlerFunc(ph.proxyData("activity"))))
	mux.Handle("GET /api/v1/activity/stats", RequireAuth(http.HandlerFunc(ph.proxyData("activity/stats"))))

	// -------------------------------------------------------------------------
	// Media (stubs — Epic 2 / image processing)
	// -------------------------------------------------------------------------

	mux.Handle("POST /api/v1/media", RequireAuth(http.HandlerFunc(notImplemented)))
	mux.Handle("GET /api/v1/media/{id}", RequireAuth(http.HandlerFunc(notImplemented)))

	// -------------------------------------------------------------------------
	// Swagger UI — /docs/
	// -------------------------------------------------------------------------

	mux.Handle("GET /docs/", httpSwagger.WrapHandler)

	return mux
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented — see Epic 2 (visual garden understanding)")
}
