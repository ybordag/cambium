package api

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ybordag/cambium/internal/rhizome"
)

type threadHandler struct {
	pool    *pgxpool.Pool
	rhizome *rhizome.Client
}

// createThread generates a botanical thread ID, registers it with Rhizome,
// and returns it to the client. The client uses it for all subsequent chat calls.
//
// POST /api/v1/threads
// Body (optional): { "title": "...", "project_id": "..." }
// Response: { "thread_id": "silver-fern-cascade" }
func (h *threadHandler) createThread(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())

	var opts struct {
		Title     string `json:"title"`
		ProjectID string `json:"project_id"`
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

	body, status, err := h.rhizome.DataPost("threads", userID, payload)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to register thread: "+err.Error())
		return
	}
	defer body.Close()

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
