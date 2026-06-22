package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ybordag/cambium/internal/auth"
	"github.com/ybordag/cambium/internal/db"
	"github.com/ybordag/cambium/internal/rhizome"
)

// proxyHandler holds dependencies for all proxy routes.
type proxyHandler struct {
	pool    *pgxpool.Pool
	rhizome *rhizome.Client
}

// newProxyHandler constructs a proxyHandler with a shared Rhizome client.
func newProxyHandler(pool *pgxpool.Pool) *proxyHandler {
	return &proxyHandler{pool: pool, rhizome: rhizome.New()}
}

// -------------------------------------------------------------------------
// Provider key injection
// -------------------------------------------------------------------------

// providerKey decrypts and returns the user's preferred provider key.
// Falls back gracefully: no preferred provider → empty strings (Rhizome
// will use its own env-var fallback for local dev).
func (h *proxyHandler) providerKey(r *http.Request, userID string) (provider, decryptedKey, model string) {
	user, err := db.GetUserByID(r.Context(), h.pool, userID)
	if err != nil || user.PreferredProvider == nil {
		return "", "", ""
	}
	provider = *user.PreferredProvider
	if user.PreferredModel != nil {
		model = *user.PreferredModel
	}

	var encryptedKey *string
	switch provider {
	case "gemini":
		encryptedKey = user.EncryptedGeminiKey
	case "openai":
		encryptedKey = user.EncryptedOpenAIKey
	case "anthropic":
		encryptedKey = user.EncryptedAnthropicKey
	}
	if encryptedKey == nil {
		return provider, "", model
	}
	key, err := auth.DecryptKey(*encryptedKey)
	if err != nil {
		return provider, "", model
	}
	return provider, key, model
}

// -------------------------------------------------------------------------
// Chat — agent proxy
// -------------------------------------------------------------------------

// chat sends a message to the Rhizome LangGraph agent and returns the complete response.
// Decrypts the user's preferred provider key before forwarding to Rhizome.
//
//	@Summary	Chat (non-streaming)
//	@Tags		chat
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		thread_id	query		string		true	"Thread ID (from POST /api/v1/threads)"
//	@Param		body		body		ChatRequest	true	"User message"
//	@Success	200			{object}	ChatResponse
//	@Failure	400			{object}	ErrorResponse
//	@Failure	401			{object}	ErrorResponse
//	@Failure	502			{object}	ErrorResponse	"Rhizome unavailable"
//	@Router		/api/v1/chat [post]
func (h *proxyHandler) chat(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	threadID := r.URL.Query().Get("thread_id")
	if threadID == "" {
		writeError(w, http.StatusBadRequest, "thread_id query parameter required")
		return
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Message == "" {
		writeError(w, http.StatusBadRequest, "message required")
		return
	}

	provider, key, model := h.providerKey(r, userID)
	resp, err := h.rhizome.RunAgent(rhizome.AgentRequest{
		UserID:      userID,
		ThreadID:    threadID,
		Message:     body.Message,
		Provider:    provider,
		ProviderKey: key,
		Model:       model,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "rhizome unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// chatStream streams LLM tokens via Server-Sent Events as they are produced.
// Event format: data: {"type":"token","content":"..."} — data: {"type":"done"}
// If the agent pauses for confirmation: data: {"type":"interaction","payload":{...}}
//
//	@Summary	Chat (SSE streaming)
//	@Tags		chat
//	@Accept		json
//	@Produce	text/event-stream
//	@Security	BearerAuth
//	@Param		thread_id	query		string		true	"Thread ID (from POST /api/v1/threads)"
//	@Param		body		body		ChatRequest	true	"User message"
//	@Success	200			{string}	string		"SSE stream of typed events"
//	@Failure	400			{object}	ErrorResponse
//	@Failure	401			{object}	ErrorResponse
//	@Failure	502			{object}	ErrorResponse
//	@Router		/api/v1/chat/stream [post]
func (h *proxyHandler) chatStream(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	threadID := r.URL.Query().Get("thread_id")
	if threadID == "" {
		writeError(w, http.StatusBadRequest, "thread_id query parameter required")
		return
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Message == "" {
		writeError(w, http.StatusBadRequest, "message required")
		return
	}

	provider, key, model := h.providerKey(r, userID)
	stream, err := h.rhizome.StreamAgent(rhizome.AgentRequest{
		UserID:      userID,
		ThreadID:    threadID,
		Message:     body.Message,
		Provider:    provider,
		ProviderKey: key,
		Model:       model,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "rhizome stream unavailable: "+err.Error())
		return
	}
	defer stream.Close()
	proxySSE(w, stream)
}

// chatResume resumes a paused interaction (non-streaming).
//
//	@Summary	Resume interaction (non-streaming)
//	@Tags		chat
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		ResumeRequestBody	true	"Thread ID and resolution value"
//	@Success	200		{object}	ChatResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/chat/resume [post]
func (h *proxyHandler) chatResume(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	var body struct {
		ThreadID   string `json:"thread_id"`
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ThreadID == "" || body.Resolution == "" {
		writeError(w, http.StatusBadRequest, "thread_id and resolution required")
		return
	}
	resp, err := h.rhizome.ResumeAgent(rhizome.ResumeRequest{
		UserID:     userID,
		ThreadID:   body.ThreadID,
		Resolution: body.Resolution,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "rhizome resume unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// chatResumeStream resumes a paused interaction with SSE streaming.
//
//	@Summary	Resume interaction (SSE streaming)
//	@Tags		chat
//	@Accept		json
//	@Produce	text/event-stream
//	@Security	BearerAuth
//	@Param		body	body		ResumeRequestBody	true	"Thread ID and resolution value"
//	@Success	200		{string}	string				"SSE stream of typed events"
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/chat/resume/stream [post]
func (h *proxyHandler) chatResumeStream(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	var body struct {
		ThreadID   string `json:"thread_id"`
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ThreadID == "" || body.Resolution == "" {
		writeError(w, http.StatusBadRequest, "thread_id and resolution required")
		return
	}
	stream, err := h.rhizome.StreamResume(rhizome.ResumeRequest{
		UserID:     userID,
		ThreadID:   body.ThreadID,
		Resolution: body.Resolution,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "rhizome stream unavailable: "+err.Error())
		return
	}
	defer stream.Close()
	proxySSE(w, stream)
}

// -------------------------------------------------------------------------
// Notifications — long-lived SSE stream (GET, not POST like chat)
// -------------------------------------------------------------------------

// notificationList proxies GET /internal/data/notifications as a synchronous
// notification snapshot. It preserves query params such as since.
//
//	@Summary	List notifications
//	@Tags		notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Param		since	query		string	false	"Only return notifications after this timestamp"
//	@Success	200		{object}	map[string]any
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/notifications [get]
func (h *proxyHandler) notificationList(w http.ResponseWriter, r *http.Request) {
	h.proxyData("notifications")(w, r)
}

// notificationStream proxies GET /internal/data/notifications/stream as a
// long-lived SSE connection. Unlike the chat stream endpoints, this is a GET
// with no request body — the verified user_id is the only required param.
// The frontend opens this once on app mount and keeps it open for the session.
//
//	@Summary	Notification stream (SSE)
//	@Tags		notifications
//	@Produce	text/event-stream
//	@Security	BearerAuth
//	@Success	200	{string}	string	"SSE stream of typed events (alert, interaction_pending, job_started/job_step/job_complete/job_failed, heartbeat)"
//	@Failure	401	{object}	ErrorResponse
//	@Failure	502	{object}	ErrorResponse
//	@Router		/api/v1/notifications/stream [get]
func (h *proxyHandler) notificationStream(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	stream, err := h.rhizome.StreamData("notifications/stream", userID, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "rhizome stream unavailable: "+err.Error())
		return
	}
	defer stream.Close()
	proxySSE(w, stream)
}

// -------------------------------------------------------------------------
// Threads — structured session context proxy
// -------------------------------------------------------------------------

// getThreadSessionContext proxies Rhizome's normalized SessionContextView.
// Verdant should use this endpoint instead of ThreadView.session_context, which
// is raw stored JSON on Rhizome thread metadata responses.
//
//	@Summary	Get thread session context
//	@Tags		threads
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Thread ID"
//	@Success	200	{object}	SessionContextView
//	@Failure	401	{object}	ErrorResponse
//	@Failure	404	{object}	ErrorResponse
//	@Failure	502	{object}	ErrorResponse
//	@Router		/api/v1/threads/{id}/session-context [get]
func (h *proxyHandler) getThreadSessionContext(w http.ResponseWriter, r *http.Request) {
	h.proxyDataWithPathParam("threads", "id").ServeHTTP(w, r)
}

// patchThreadSessionContext proxies user overrides for Rhizome's normalized
// SessionContextView. Explicit nulls clear nullable fields.
//
//	@Summary	Update thread session context
//	@Tags		threads
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string						true	"Thread ID"
//	@Param		body	body	UpdateSessionContextRequest	true	"Partial session context override"
//	@Success	200		{object}	SessionContextView
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	404		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/threads/{id}/session-context [patch]
func (h *proxyHandler) patchThreadSessionContext(w http.ResponseWriter, r *http.Request) {
	h.proxyDataWithPathParam("threads", "id").ServeHTTP(w, r)
}

// -------------------------------------------------------------------------
// Data proxy — CRUD pass-through
// -------------------------------------------------------------------------

// dispatchData calls the right Rhizome client method for the request's HTTP
// method: GET forwards query params, DELETE sends no body, everything else
// (POST/PATCH/PUT) decodes and forwards the JSON body under its own verb.
func (h *proxyHandler) dispatchData(r *http.Request, dataPath, userID string) (io.ReadCloser, int, error) {
	switch r.Method {
	case http.MethodGet:
		return h.rhizome.DataGet(dataPath, userID, r.URL.Query())
	case http.MethodDelete:
		return h.rhizome.DataDelete(dataPath, userID)
	default:
		var payload any
		json.NewDecoder(r.Body).Decode(&payload)
		return h.rhizome.DataRequest(r.Method, dataPath, userID, nil, payload)
	}
}

// proxyData proxies a request to Rhizome's /internal/data/{path} endpoint,
// preserving the original HTTP method (GET/POST/PATCH/DELETE).
func (h *proxyHandler) proxyData(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		body, status, err := h.dispatchData(r, path, userID)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		defer body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.Copy(w, body)
	}
}

// proxyDataWithPathParam handles routes with one or more path segments after
// the registered prefix (e.g. {id}, or {id}/context/{type}/{subjectId}). It
// forwards the full path suffix verbatim and preserves the original HTTP method.
func (h *proxyHandler) proxyDataWithPathParam(prefix, paramName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build: prefix + "/" + paramValue + remaining suffix
		fullPath := strings.TrimSuffix(r.URL.Path, "/")
		// Strip the /api/v1/ prefix to get the data path
		dataPath := strings.TrimPrefix(fullPath, "/api/v1/")

		userID, _ := UserIDFromContext(r.Context())
		body, status, err := h.dispatchData(r, dataPath, userID)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		defer body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.Copy(w, body)
	}
}

// -------------------------------------------------------------------------
// SSE proxy helper
// -------------------------------------------------------------------------

// proxySSE forwards an SSE stream from Rhizome to the HTTP client with
// appropriate headers and flushing.
func proxySSE(w http.ResponseWriter, stream io.Reader) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if present

	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}
