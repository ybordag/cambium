package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/ybordag/cambium/internal/auth"
	"github.com/ybordag/cambium/internal/db"
	"github.com/ybordag/cambium/internal/rhizome"
	"github.com/jackc/pgx/v5/pgxpool"
)

// proxyHandler holds dependencies for all proxy routes.
type proxyHandler struct {
	pool   *pgxpool.Pool
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
// Data proxy — CRUD pass-through
// -------------------------------------------------------------------------

// proxyData proxies a request to Rhizome's /internal/data/{path} endpoint.
// GET requests forward query params; POST requests forward the body.
func (h *proxyHandler) proxyData(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())

		var (
			body   io.ReadCloser
			status int
			err    error
		)

		if r.Method == http.MethodGet {
			params := r.URL.Query()
			body, status, err = h.rhizome.DataGet(path, userID, params)
		} else {
			var payload any
			json.NewDecoder(r.Body).Decode(&payload)
			body, status, err = h.rhizome.DataPost(path, userID, payload)
		}

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

// proxyDataWithPathParam handles routes with a path parameter like {id}.
// It appends the path value segment to the Rhizome data path.
func (h *proxyHandler) proxyDataWithPathParam(prefix, paramName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paramValue := r.PathValue(paramName)
		// Build: prefix + "/" + paramValue + remaining suffix
		fullPath := strings.TrimSuffix(r.URL.Path, "/")
		// Strip the /api/v1/ prefix to get the data path
		dataPath := strings.TrimPrefix(fullPath, "/api/v1/")
		_ = prefix
		_ = paramValue

		userID, _ := UserIDFromContext(r.Context())
		var (
			body   io.ReadCloser
			status int
			err    error
		)

		if r.Method == http.MethodGet {
			params := r.URL.Query()
			body, status, err = h.rhizome.DataGet(dataPath, userID, params)
		} else {
			var payload any
			json.NewDecoder(r.Body).Decode(&payload)
			body, status, err = h.rhizome.DataPost(dataPath, userID, payload)
		}

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
