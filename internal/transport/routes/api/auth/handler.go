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

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/httpx"
	"github.com/sarga-app/sarga/internal/transport/routes/api/gen"
)

// Compile-time assertion: Handler must implement every operation defined in
// api/openapi.yaml. Spec changes break the build until handlers catch up.
var _ gen.ServerInterface = (*Handler)(nil)

type Handler struct {
	store Store
	jwt   domauth.JWTConfig
}

func NewHandler(store Store, jwt domauth.JWTConfig) *Handler {
	return &Handler{store: store, jwt: jwt}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req gen.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "username and password required")
		return
	}

	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = domauth.Verify(req.Password, dummyHash) //nolint:errcheck // intentional: only side effect (CPU time) matters
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
		slog.Error("login: lookup user", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if user.DisabledAt != nil || user.PasswordHash == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	ok, err := domauth.Verify(req.Password, *user.PasswordHash)
	if err != nil || !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	if domauth.NeedsRehash(*user.PasswordHash) {
		if newHash, hErr := domauth.Hash(req.Password); hErr == nil {
			if uErr := h.store.UpdateUserPassword(r.Context(), queries.UpdateUserPasswordParams{
				ID: user.ID, PasswordHash: &newHash,
			}); uErr != nil {
				slog.Warn("login: rehash upgrade failed", "user_id", user.ID, "err", uErr)
			}
		}
	}

	if err := h.issueSession(w, r, user, nil); err != nil {
		slog.Error("login: issue session", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "missing_refresh", "refresh token missing")
		return
	}
	hash, err := domauth.HashRefreshToken(cookie.Value)
	if err != nil {
		clearRefreshCookie(w, r)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token invalid")
		return
	}
	row, err := h.store.GetRefreshTokenByHash(r.Context(), hash)
	if err != nil {
		clearRefreshCookie(w, r)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token invalid")
		return
	}
	if row.RevokedAt != nil {
		// Replay detected: nuke all active sessions for this user.
		if rErr := h.store.RevokeAllUserRefreshTokens(r.Context(), row.UserID); rErr != nil {
			slog.Error("refresh: revoke-all on replay failed", "user_id", row.UserID, "err", rErr)
		}
		clearRefreshCookie(w, r)
		slog.Warn("refresh: replay detected", "user_id", row.UserID, "token_id", row.ID)
		httpx.WriteError(w, http.StatusUnauthorized, "replay_detected", "session terminated")
		return
	}
	if time.Now().After(row.ExpiresAt) {
		clearRefreshCookie(w, r)
		httpx.WriteError(w, http.StatusUnauthorized, "expired_refresh", "refresh token expired")
		return
	}

	user, err := h.store.GetUserByID(r.Context(), row.UserID)
	if err != nil || user.DisabledAt != nil {
		clearRefreshCookie(w, r)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	if user.PasswordChangedAt != nil && row.CreatedAt.Before(*user.PasswordChangedAt) {
		if rErr := h.store.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
			slog.Warn("refresh: revoke after password change failed", "token_id", row.ID, "err", rErr)
		}
		clearRefreshCookie(w, r)
		httpx.WriteError(w, http.StatusUnauthorized, "password_changed", "session invalidated")
		return
	}

	if rErr := h.store.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
		slog.Error("refresh: revoke old", "err", rErr)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if err := h.issueSession(w, r, user, &row.ID); err != nil {
		slog.Error("refresh: issue session", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid, _ := domauth.UserIDFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, gen.MeResponse{
		UserId:  uid,
		IsAdmin: domauth.IsAdminFrom(r.Context()),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.tryRevokeFromCookie(r)
	clearRefreshCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) tryRevokeFromCookie(r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	hash, err := domauth.HashRefreshToken(cookie.Value)
	if err != nil {
		return
	}
	row, err := h.store.GetRefreshTokenByHash(r.Context(), hash)
	if err != nil {
		return
	}
	if rErr := h.store.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
		slog.Warn("logout: revoke failed", "token_id", row.ID, "err", rErr)
	}
}

func (h *Handler) issueSession(w http.ResponseWriter, r *http.Request, user queries.User, parentID *uuid.UUID) error {
	access, err := domauth.IssueAccessToken(h.jwt, user.ID, user.IsAdmin)
	if err != nil {
		return err
	}
	refresh, err := domauth.IssueRefreshToken()
	if err != nil {
		return err
	}
	if _, err := h.store.CreateRefreshToken(r.Context(), queries.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: refresh.Hash,
		ExpiresAt: time.Now().Add(domauth.RefreshTTL),
		ParentID:  parentID,
	}); err != nil {
		return err
	}
	setRefreshCookie(w, r, refresh.Plaintext)
	httpx.WriteJSON(w, http.StatusOK, gen.LoginResponse{
		AccessToken: access,
		ExpiresIn:   int(domauth.AccessTTL.Seconds()),
	})
	return nil
}

// dummyHash defeats username-enumeration via login-latency timing.
var dummyHash = mustDummyHash()

func mustDummyHash() string {
	h, err := domauth.Hash("dummy-password-for-timing-purposes-only-not-a-secret")
	if err != nil {
		panic(err)
	}
	return h
}
