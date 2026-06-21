package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ybordag/cambium/internal/rhizome"
)

type threadHandler struct {
	pool    *pgxpool.Pool
	rhizome *rhizome.Client
}

// rhizomeErrorDetail reads a Rhizome error response body and extracts the
// FastAPI {"detail": "..."} message, falling back to a generic string.
func rhizomeErrorDetail(body io.Reader) string {
	var errBody struct {
		Detail string `json:"detail"`
	}
	json.NewDecoder(body).Decode(&errBody)
	if errBody.Detail != "" {
		return errBody.Detail
	}
	return "rhizome rejected the request"
}

// createThread generates a botanical three-word thread ID, registers it with
// Rhizome, and returns it. Use this before the first chat message.
//
//	@Summary	Create conversation thread
//	@Tags		threads
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		CreateThreadRequest	false	"Optional title, project link, and initial pinned context"
//	@Success	200		{object}	ThreadIDResponse	"Botanical thread ID e.g. silver-fern-cascade"
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	502		{object}	ErrorResponse
//	@Router		/api/v1/threads [post]
func (h *threadHandler) createThread(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())

	var opts struct {
		Title          string           `json:"title"`
		ProjectID      string           `json:"project_id"`
		InitialContext []map[string]any `json:"initial_context"`
	}
	json.NewDecoder(r.Body).Decode(&opts) // optional body — ignore decode errors

	threadID := generateThreadID()

	// Register with Rhizome. Retry once on the rare collision.
	payload := map[string]any{
		"thread_id": threadID,
	}
	if opts.Title != "" {
		payload["title"] = opts.Title
	}
	if opts.ProjectID != "" {
		payload["project_id"] = opts.ProjectID
	}
	if opts.InitialContext != nil {
		payload["initial_context"] = opts.InitialContext
	}

	body, status, err := h.rhizome.DataPost("threads", userID, payload)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to register thread: "+err.Error())
		return
	}
	defer body.Close()

	if status == http.StatusBadRequest {
		writeError(w, http.StatusBadRequest, rhizomeErrorDetail(body))
		return
	}
	if status != http.StatusOK {
		writeError(w, http.StatusBadGateway, "rhizome returned non-200 registering thread")
		return
	}

	var result map[string]any
	json.NewDecoder(body).Decode(&result)

	// If the ID collided (created=false), generate a new one and retry once.
	if created, ok := result["created"].(bool); ok && !created {
		threadID = generateThreadID()
		payload["thread_id"] = threadID
		retryBody, retryStatus, retryErr := h.rhizome.DataPost("threads", userID, payload)
		if retryErr != nil || retryStatus != http.StatusOK {
			writeError(w, http.StatusBadGateway, "failed to register thread after retry")
			return
		}
		defer retryBody.Close()
	}

	writeJSON(w, http.StatusOK, map[string]string{"thread_id": threadID})
}
