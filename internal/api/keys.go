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

// set stores or updates an encrypted provider API key.
// Keys are encrypted with AES-256-GCM and never returned to the client.
//
//	@Summary	Set provider key
//	@Tags		keys
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		SetKeyRequest	true	"Provider and API key"
//	@Success	200		{object}	map[string]string
//	@Failure	400		{object}	ErrorResponse	"Invalid provider — must be gemini, openai, or anthropic"
//	@Failure	401		{object}	ErrorResponse
//	@Router		/api/v1/auth/keys [put]
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

// list returns which providers have a key configured (booleans only — never actual key values).
//
//	@Summary	List configured providers
//	@Tags		keys
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	KeysResponse
//	@Failure	401	{object}	ErrorResponse
//	@Router		/api/v1/auth/keys [get]
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

// delete removes a stored provider key.
//
//	@Summary	Delete provider key
//	@Tags		keys
//	@Produce	json
//	@Security	BearerAuth
//	@Param		provider	path		string			true	"Provider name"	Enums(gemini, openai, anthropic)
//	@Success	200			{object}	KeyDeletedResponse
//	@Failure	400			{object}	ErrorResponse
//	@Failure	401			{object}	ErrorResponse
//	@Router		/api/v1/auth/keys/{provider} [delete]
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
