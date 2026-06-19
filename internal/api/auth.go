package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ybordag/cambium/internal/auth"
	"github.com/ybordag/cambium/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authHandler struct {
	pool *pgxpool.Pool
}

// register creates a new user account and returns tokens.
//
//	@Summary	Register
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		RegisterRequest	true	"Email and password"
//	@Success	200		{object}	TokenResponse	"Access token in body; refresh token in httpOnly cookie"
//	@Failure	400		{object}	ErrorResponse
//	@Failure	409		{object}	ErrorResponse	"Email already registered"
//	@Router		/auth/register [post]
func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	userID, err := db.InsertUser(r.Context(), h.pool, body.Email, hash)
	if errors.Is(err, db.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.issueTokenPair(w, r, userID)
}

// login authenticates an existing user and returns tokens.
//
//	@Summary	Login
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		LoginRequest	true	"Email and password"
//	@Success	200		{object}	TokenResponse	"Access token in body; refresh token in httpOnly cookie"
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse	"Invalid credentials"
//	@Router		/auth/login [post]
func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := db.GetUserByEmail(r.Context(), h.pool, body.Email)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, body.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.issueTokenPair(w, r, user.ID)
}

// refresh rotates the refresh token and returns a new access token.
//
//	@Summary	Refresh tokens
//	@Tags		auth
//	@Produce	json
//	@Success	200	{object}	TokenResponse	"New access token; new refresh token in httpOnly cookie"
//	@Failure	401	{object}	ErrorResponse	"Refresh token missing, expired, or revoked"
//	@Router		/auth/refresh [post]
func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token cookie missing")
		return
	}

	tokenHash := db.HashToken(cookie.Value)
	stored, err := db.GetRefreshToken(r.Context(), h.pool, tokenHash)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "refresh token expired or revoked")
		return
	}

	// Rotate: revoke old, issue new
	if err := db.RevokeRefreshToken(r.Context(), h.pool, stored.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate token")
		return
	}

	h.issueTokenPair(w, r, stored.UserID)
}

// session returns the authenticated user's profile.
//
//	@Summary	Session
//	@Tags		auth
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	SessionResponse
//	@Failure	401	{object}	ErrorResponse
//	@Router		/auth/session [get]
func (h *authHandler) session(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	user, err := db.GetUserByID(r.Context(), h.pool, userID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":            user.ID,
		"email":              user.Email,
		"preferred_provider": user.PreferredProvider,
		"preferred_model":    user.PreferredModel,
	})
}

// logout revokes the current refresh token.
//
//	@Summary	Logout
//	@Tags		auth
//	@Produce	json
//	@Success	200	{object}	LogoutResponse
//	@Router		/auth/logout [post]
func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		tokenHash := db.HashToken(cookie.Value)
		if stored, err := db.GetRefreshToken(r.Context(), h.pool, tokenHash); err == nil {
			db.RevokeRefreshToken(r.Context(), h.pool, stored.ID)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Path:     "/",
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// issueTokenPair issues a new access token and refresh token, sets the cookie,
// and writes the access token to the response body.
func (h *authHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, userID string) {
	accessToken, err := auth.IssueAccessToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	rawRefresh, err := db.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	expiresAt := time.Now().Add(auth.RefreshTokenDuration())
	if _, err := db.InsertRefreshToken(r.Context(), h.pool, userID, db.HashToken(rawRefresh), expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store refresh token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rawRefresh,
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}
