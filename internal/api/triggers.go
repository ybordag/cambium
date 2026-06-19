package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ybordag/cambium/internal/rhizome"
)

// triggerTriage sends a pre-built triage request to the Rhizome agent.
// POST /api/v1/triage/run  { "thread_id": "..." }
// triggerTriage asks the agent to run daily triage and return task recommendations.
//
//	@Summary	Run daily triage (AI)
//	@Tags		triage
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		ResumeRequestBody	true	"thread_id required"
//	@Success	200		{object}	ChatResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/triage/run [post]
func (h *proxyHandler) triggerTriage(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	threadID := threadIDFromBody(r)
	if threadID == "" {
		writeError(w, http.StatusBadRequest, "thread_id required")
		return
	}
	provider, key, model := h.providerKey(r, userID)
	resp, err := h.rhizome.RunAgent(rhizome.AgentRequest{
		UserID:      userID,
		ThreadID:    threadID,
		Message:     "Run daily triage now and summarise the most urgent tasks.",
		Provider:    provider,
		ProviderKey: key,
		Model:       model,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "triage failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// triggerWeatherDraft sends a weather impact draft request to the agent.
// POST /api/v1/weather/tasks/draft  { "thread_id": "..." }
// triggerWeatherDraft asks the agent to draft weather-driven task changes.
//
//	@Summary	Draft weather task changes (AI)
//	@Tags		weather
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		ResumeRequestBody	true	"thread_id required"
//	@Success	200		{object}	ChatResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/weather/tasks/draft [post]
func (h *proxyHandler) triggerWeatherDraft(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	threadID := threadIDFromBody(r)
	if threadID == "" {
		writeError(w, http.StatusBadRequest, "thread_id required")
		return
	}
	provider, key, model := h.providerKey(r, userID)
	resp, err := h.rhizome.RunAgent(rhizome.AgentRequest{
		UserID:      userID,
		ThreadID:    threadID,
		Message:     "Review the latest weather forecast and draft any recommended task changes.",
		Provider:    provider,
		ProviderKey: key,
		Model:       model,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "weather draft failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// triggerTreatmentDraft asks the agent to draft a treatment plan for an incident.
// POST /api/v1/incidents/{id}/treatment  { "thread_id": "..." }
// triggerTreatmentDraft asks the agent to draft a treatment plan for an incident.
//
//	@Summary	Draft treatment plan (AI)
//	@Tags		incidents
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string				true	"Incident ID"
//	@Param		body	body		ResumeRequestBody	true	"thread_id required"
//	@Success	200		{object}	ChatResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/incidents/{id}/treatment [post]
func (h *proxyHandler) triggerTreatmentDraft(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	incidentID := r.PathValue("id")
	threadID := threadIDFromBody(r)
	if threadID == "" {
		writeError(w, http.StatusBadRequest, "thread_id required")
		return
	}
	provider, key, model := h.providerKey(r, userID)
	resp, err := h.rhizome.RunAgent(rhizome.AgentRequest{
		UserID:      userID,
		ThreadID:    threadID,
		Message:     fmt.Sprintf("Draft a treatment plan for incident %s.", incidentID),
		Provider:    provider,
		ProviderKey: key,
		Model:       model,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "treatment draft failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// triggerTaskGeneration asks the agent to generate tasks for a project.
// POST /api/v1/projects/{id}/tasks/generate  { "thread_id": "..." }
// triggerTaskGeneration asks the agent to generate the initial task plan for a project.
//
//	@Summary	Generate project tasks (AI)
//	@Tags		projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string				true	"Project ID"
//	@Param		body	body		ResumeRequestBody	true	"thread_id required"
//	@Success	200		{object}	ChatResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/projects/{id}/tasks/generate [post]
func (h *proxyHandler) triggerTaskGeneration(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	projectID := r.PathValue("id")
	threadID := threadIDFromBody(r)
	if threadID == "" {
		writeError(w, http.StatusBadRequest, "thread_id required")
		return
	}
	provider, key, model := h.providerKey(r, userID)
	resp, err := h.rhizome.RunAgent(rhizome.AgentRequest{
		UserID:      userID,
		ThreadID:    threadID,
		Message:     fmt.Sprintf("Generate the initial task plan for project %s.", projectID),
		Provider:    provider,
		ProviderKey: key,
		Model:       model,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "task generation failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// threadIDFromBody reads thread_id from a JSON request body.
// Returns empty string if the body is missing or malformed.
func threadIDFromBody(r *http.Request) string {
	var body struct {
		ThreadID string `json:"thread_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	return body.ThreadID
}
