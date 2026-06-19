package api

import (
	"encoding/json"
	"net/http"

	"github.com/ybordag/cambium/internal/auth"
	"github.com/ybordag/cambium/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type keysHandler struct {
	pool *pgxpool.Pool
}

func (h *keysHandler) set(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var body struct {
		Provider string `json:"provider"`
		Key      string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Provider == "" || body.Key == "" {
		writeError(w, http.StatusBadRequest, "provider and key required")
		return
	}

	encrypted, err := auth.EncryptKey(body.Key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt key")
		return
	}

	if err := db.SetProviderKey(r.Context(), h.pool, userID, body.Provider, encrypted); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "key stored"})
}

func (h *keysHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := db.GetUserByID(r.Context(), h.pool, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{
		"gemini":    user.EncryptedGeminiKey != nil,
		"openai":    user.EncryptedOpenAIKey != nil,
		"anthropic": user.EncryptedAnthropicKey != nil,
	})
}

func (h *keysHandler) delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	provider := r.PathValue("provider")
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider path parameter required")
		return
	}

	if err := db.ClearProviderKey(r.Context(), h.pool, userID, provider); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "key removed"})
}
