package api

// Request/response types used only for Swagger schema generation.
// Actual handler logic uses inline anonymous structs or map[string]any —
// these named types give swaggo something to reference in annotations.

// --- Auth ---

// RegisterRequest is the body for POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email" example:"you@example.com"`
	Password string `json:"password" example:"correct-horse-battery-staple"`
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" example:"you@example.com"`
	Password string `json:"password" example:"correct-horse-battery-staple"`
}

// TokenResponse is returned by register, login, and refresh.
type TokenResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// SessionResponse is returned by GET /auth/session.
type SessionResponse struct {
	UserID            string  `json:"user_id" example:"abc-123-uuid"`
	Email             string  `json:"email" example:"you@example.com"`
	PreferredProvider *string `json:"preferred_provider,omitempty" example:"gemini"`
	PreferredModel    *string `json:"preferred_model,omitempty" example:"gemini-2.5-flash"`
}

// LogoutResponse is returned by POST /auth/logout.
type LogoutResponse struct {
	Status string `json:"status" example:"logged out"`
}

// --- Key management ---

// SetKeyRequest is the body for PUT /api/v1/auth/keys.
type SetKeyRequest struct {
	Provider string `json:"provider" example:"gemini" enums:"gemini,openai,anthropic"`
	Key      string `json:"key" example:"AIzaSy..."`
}

// KeysResponse is returned by GET /api/v1/auth/keys.
// Never returns actual key values — only booleans indicating which are configured.
type KeysResponse struct {
	Gemini    bool `json:"gemini" example:"true"`
	OpenAI    bool `json:"openai" example:"false"`
	Anthropic bool `json:"anthropic" example:"false"`
}

// KeyDeletedResponse is returned by DELETE /api/v1/auth/keys/{provider}.
type KeyDeletedResponse struct {
	Status string `json:"status" example:"key removed"`
}

// --- Chat ---

// ChatRequest is the body for POST /api/v1/chat and /chat/stream.
type ChatRequest struct {
	Message string `json:"message" example:"What should I do in my garden today?"`
}

// ChatResponse is returned by POST /api/v1/chat (non-streaming).
type ChatResponse struct {
	ThreadID    string         `json:"thread_id" example:"silver-fern-cascade"`
	Response    string         `json:"response" example:"Based on your garden profile..."`
	Interaction map[string]any `json:"interaction,omitempty"`
}

// ResumeRequest is the body for POST /api/v1/chat/resume.
type ResumeRequestBody struct {
	ThreadID   string `json:"thread_id" example:"silver-fern-cascade"`
	Resolution string `json:"resolution" example:"confirm"`
}

// --- Threads ---

// CreateThreadRequest is the optional body for POST /api/v1/threads.
type CreateThreadRequest struct {
	Title          string                    `json:"title,omitempty" example:"Spring tomato project"`
	ProjectID      string                    `json:"project_id,omitempty" example:"proj-uuid"`
	InitialContext []PinnedContextEntryInput `json:"initial_context,omitempty"`
}

// ThreadIDResponse is returned by POST /api/v1/threads.
type ThreadIDResponse struct {
	ThreadID string `json:"thread_id" example:"silver-fern-cascade"`
}

// PinnedContextEntryInput is one entity reference in a pinned-context request.
type PinnedContextEntryInput struct {
	SubjectType string `json:"subject_type" example:"plant"`
	SubjectID   string `json:"subject_id" example:"plant-uuid"`
}

// PinnedContextResponse is returned by the thread context endpoints.
type PinnedContextResponse struct {
	ThreadID      string                    `json:"thread_id" example:"silver-fern-cascade"`
	PinnedContext []PinnedContextEntryInput `json:"pinned_context"`
}

// SessionContextView is the normalized session context returned by
// GET/PATCH /api/v1/threads/{id}/session-context.
type SessionContextView struct {
	TimeText     *string                   `json:"time_text,omitempty" example:"45 minutes, all afternoon"`
	EnergyText   *string                   `json:"energy_text,omitempty" example:"low, focused, tired but can water"`
	FocusText    *string                   `json:"focus_text,omitempty" example:"How do I fertilize the cherry tomatoes?"`
	FocusContext []SessionContextObjectRef `json:"focus_context,omitempty"`
	Source       string                    `json:"source" example:"user" enums:"unset,inferred,user"`
	UpdatedAt    *string                   `json:"updated_at,omitempty" example:"2026-06-25T12:00:00Z"`
}

type SessionContextObjectRef struct {
	SubjectType string  `json:"subject_type" example:"plant"`
	SubjectID   string  `json:"subject_id" example:"plant-uuid"`
	Label       *string `json:"label,omitempty" example:"Cherry Tomato (Sungold)"`
}

// UpdateSessionContextRequest is a partial override body for PATCH
// /api/v1/threads/{id}/session-context. Explicit JSON null clears nullable
// fields; omitted fields are left unchanged.
type UpdateSessionContextRequest struct {
	TimeText     *string                   `json:"time_text,omitempty" example:"45 minutes, all afternoon"`
	EnergyText   *string                   `json:"energy_text,omitempty" example:"low, focused, tired but can water"`
	FocusText    *string                   `json:"focus_text,omitempty" example:"How do I fertilize the cherry tomatoes?"`
	FocusContext []SessionContextObjectRef `json:"focus_context,omitempty"`
}

// --- Errors ---

// ErrorResponse is returned on all 4xx/5xx responses.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid or expired token"`
}

// --- Health ---

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}
