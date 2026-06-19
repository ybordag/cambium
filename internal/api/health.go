package api

import (
	"encoding/json"
	"net/http"
)

// healthHandler checks service health.
//
//	@Summary	Health check
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	HealthResponse
//	@Router		/health [get]
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
