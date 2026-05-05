package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	coreauth "github.com/sarga-app/sarga/internal/core/auth"
	"github.com/sarga-app/sarga/internal/platform/db/queries"
)

type Handler struct {
	Store        Store
	JWT          coreauth.JWTConfig
	RefreshTTL   time.Duration
	CookieSecure bool
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"` //nolint:gosec // OAuth-style API contract
	ExpiresIn   int    `json:"expires_in"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "username and password required")
		return
	}

	user, err := h.Store.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Dummy verify keeps timing constant whether or not the username exists.
			_, _ = coreauth.Verify(req.Password, dummyHash) //nolint:errcheck // intentional: only side effect (CPU time) matters
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
		slog.Error("login: lookup user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if user.DisabledAt != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	if user.PasswordHash == nil {
		writeError(w, http.StatusUnauthorized, "no_password", "this account does not have a password set; sign in via SSO")
		return
	}

	ok, err := coreauth.Verify(req.Password, *user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	if coreauth.NeedsRehash(*user.PasswordHash) {
		newHash, hErr := coreauth.Hash(req.Password)
		if hErr == nil {
			if uErr := h.Store.UpdateUserPassword(r.Context(), queries.UpdateUserPasswordParams{
				ID: user.ID, PasswordHash: &newHash,
			}); uErr != nil {
				slog.Warn("login: rehash upgrade failed", "user_id", user.ID, "err", uErr)
			}
		}
	}

	if err := h.issueSession(w, r, user, nil); err != nil {
		slog.Error("login: issue session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "missing_refresh", "refresh token missing")
		return
	}
	hash, err := coreauth.HashRefreshToken(cookie.Value)
	if err != nil {
		clearRefreshCookie(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token invalid")
		return
	}
	row, err := h.Store.GetRefreshTokenByHash(r.Context(), hash)
	if err != nil {
		clearRefreshCookie(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token invalid")
		return
	}
	now := time.Now()
	if row.RevokedAt != nil {
		// Replay detected: nuke all active sessions for this user.
		if rErr := h.Store.RevokeAllUserRefreshTokens(r.Context(), row.UserID); rErr != nil {
			slog.Error("refresh: revoke-all on replay failed", "user_id", row.UserID, "err", rErr)
		}
		clearRefreshCookie(w, h.CookieSecure)
		slog.Warn("refresh: replay detected", "user_id", row.UserID, "token_id", row.ID)
		writeError(w, http.StatusUnauthorized, "replay_detected", "session terminated")
		return
	}
	if now.After(row.ExpiresAt) {
		clearRefreshCookie(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "expired_refresh", "refresh token expired")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), row.UserID)
	if err != nil || user.DisabledAt != nil {
		clearRefreshCookie(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	if user.PasswordChangedAt != nil && row.CreatedAt.Before(*user.PasswordChangedAt) {
		if rErr := h.Store.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
			slog.Warn("refresh: revoke after password change failed", "token_id", row.ID, "err", rErr)
		}
		clearRefreshCookie(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "password_changed", "session invalidated")
		return
	}

	if revokeErr := h.Store.RevokeRefreshToken(r.Context(), row.ID); revokeErr != nil {
		slog.Error("refresh: revoke old", "err", revokeErr)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if err := h.issueSession(w, r, user, &row.ID); err != nil {
		slog.Error("refresh: issue session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid, _ := coreauth.UserIDFrom(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user_id":  uid,
		"is_admin": coreauth.IsAdminFrom(r.Context()),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err == nil && cookie.Value != "" {
		if hash, hErr := coreauth.HashRefreshToken(cookie.Value); hErr == nil {
			if row, gErr := h.Store.GetRefreshTokenByHash(r.Context(), hash); gErr == nil {
				if rErr := h.Store.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
					slog.Warn("logout: revoke failed", "token_id", row.ID, "err", rErr)
				}
			}
		}
	}
	clearRefreshCookie(w, h.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) issueSession(w http.ResponseWriter, r *http.Request, user queries.User, parentID *uuid.UUID) error {
	access, err := coreauth.IssueAccessToken(h.JWT, user.ID, user.IsAdmin)
	if err != nil {
		return err
	}
	refresh, err := coreauth.IssueRefreshToken()
	if err != nil {
		return err
	}
	if _, err := h.Store.CreateRefreshToken(r.Context(), queries.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: refresh.Hash,
		ExpiresAt: time.Now().Add(h.RefreshTTL),
		ParentID:  parentID,
	}); err != nil {
		return err
	}
	setRefreshCookie(w, refresh.Plaintext, h.RefreshTTL, h.CookieSecure)
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(loginResponse{
		AccessToken: access,
		ExpiresIn:   int(h.JWT.AccessTTL.Seconds()),
	})
}

// dummyHash defeats username-enumeration via login-latency timing.
var dummyHash = mustDummyHash()

func mustDummyHash() string {
	h, err := coreauth.Hash("dummy-password-for-timing-purposes-only-not-a-secret")
	if err != nil {
		panic(err)
	}
	return h
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorBody{Error: errorPayload{Code: code, Message: msg}}); err != nil {
		slog.Error("write error response", "err", err)
	}
}
