package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/auth"
	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/openapi"
)

const (
	authBodyLimit     = 4 * 1024
	refreshCookieName = "sarga_refresh"
	refreshCookiePath = "/api/v1/auth"
)

type authStore interface {
	GetUserByUsername(ctx context.Context, username string) (queries.User, error)
	GetUserByID(ctx context.Context, id string) (queries.User, error)
	UpdateUserPassword(ctx context.Context, arg queries.UpdateUserPasswordParams) error

	CreateRefreshToken(ctx context.Context, arg queries.CreateRefreshTokenParams) (queries.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (queries.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
}

type authStoreImpl struct {
	q *queries.Queries
}

func buildAuthStore(d *db.DB) authStore {
	return &authStoreImpl{q: queries.New(d.Write())}
}

func (s *authStoreImpl) GetUserByUsername(ctx context.Context, username string) (queries.User, error) {
	return s.q.GetUserByUsername(ctx, username)
}

func (s *authStoreImpl) GetUserByID(ctx context.Context, id string) (queries.User, error) {
	return s.q.GetUserByID(ctx, id)
}

func (s *authStoreImpl) UpdateUserPassword(ctx context.Context, arg queries.UpdateUserPasswordParams) error {
	return s.q.UpdateUserPassword(ctx, arg)
}

func (s *authStoreImpl) CreateRefreshToken(ctx context.Context, arg queries.CreateRefreshTokenParams) (queries.RefreshToken, error) {
	return s.q.CreateRefreshToken(ctx, arg)
}

func (s *authStoreImpl) GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (queries.RefreshToken, error) {
	return s.q.GetRefreshTokenByHash(ctx, tokenHash)
}

func (s *authStoreImpl) RevokeRefreshToken(ctx context.Context, id string) error {
	return s.q.RevokeRefreshToken(ctx, id)
}

func (s *authStoreImpl) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	return s.q.RevokeAllUserRefreshTokens(ctx, userID)
}

func (a *API) mountAuthRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Use(bodyLimit(authBodyLimit))
		r.Post("/login", a.handleLogin)
		r.Post("/refresh", a.handleRefresh)
		r.Post("/logout", a.handleLogout)
	})

	r.Group(func(r chi.Router) {
		r.Use(requireAuth(a.jwt))
		r.Get("/me", a.handleMe)
	})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req openapi.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "username and password required")
		return
	}

	user, err := a.authStore.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, _ = auth.Verify(req.Password, dummyHash) //nolint:errcheck // intentional: only side effect (CPU time) matters
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
		slog.Error("login: lookup user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if user.DisabledAt != nil || user.PasswordHash == nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	ok, err := auth.Verify(req.Password, *user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	if auth.NeedsRehash(*user.PasswordHash) {
		if newHash, hErr := auth.Hash(req.Password); hErr == nil {
			if uErr := a.authStore.UpdateUserPassword(r.Context(), queries.UpdateUserPasswordParams{
				ID: user.ID, PasswordHash: &newHash,
			}); uErr != nil {
				slog.Warn("login: rehash upgrade failed", "user_id", user.ID, "err", uErr)
			}
		}
	}

	if err := a.issueSession(w, r, user, nil); err != nil {
		slog.Error("login: issue session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	slog.Info("login: success", "user_id", user.ID, "username", user.Username)
}

func (a *API) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "missing_refresh", "refresh token missing")
		return
	}
	hash, err := auth.HashRefreshToken(cookie.Value)
	if err != nil {
		clearRefreshCookie(w, r)
		writeError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token invalid")
		return
	}
	row, err := a.authStore.GetRefreshTokenByHash(r.Context(), hash)
	if err != nil {
		clearRefreshCookie(w, r)
		writeError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token invalid")
		return
	}
	if row.RevokedAt != nil {
		if rErr := a.authStore.RevokeAllUserRefreshTokens(r.Context(), row.UserID); rErr != nil {
			slog.Error("refresh: revoke-all on replay failed", "user_id", row.UserID, "err", rErr)
		}
		clearRefreshCookie(w, r)
		slog.Warn("refresh: replay detected", "user_id", row.UserID, "token_id", row.ID)
		writeError(w, http.StatusUnauthorized, "replay_detected", "session terminated")
		return
	}
	if time.Now().After(parseRFC3339(row.ExpiresAt)) {
		clearRefreshCookie(w, r)
		writeError(w, http.StatusUnauthorized, "expired_refresh", "refresh token expired")
		return
	}

	user, err := a.authStore.GetUserByID(r.Context(), row.UserID)
	if err != nil || user.DisabledAt != nil {
		clearRefreshCookie(w, r)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	if user.PasswordChangedAt != nil && parseRFC3339(row.CreatedAt).Before(parseRFC3339(*user.PasswordChangedAt)) {
		if rErr := a.authStore.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
			slog.Warn("refresh: revoke after password change failed", "token_id", row.ID, "err", rErr)
		}
		clearRefreshCookie(w, r)
		writeError(w, http.StatusUnauthorized, "password_changed", "session invalidated")
		return
	}

	if rErr := a.authStore.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
		slog.Error("refresh: revoke old", "err", rErr)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if err := a.issueSession(w, r, user, &row.ID); err != nil {
		slog.Error("refresh: issue session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	slog.Debug("refresh: token rotated", "user_id", user.ID)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFrom(r.Context())
	writeJSON(w, http.StatusOK, openapi.MeResponse{
		UserId:  uid,
		IsAdmin: auth.IsAdminFrom(r.Context()),
	})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.tryRevokeFromCookie(r)
	clearRefreshCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) tryRevokeFromCookie(r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	hash, err := auth.HashRefreshToken(cookie.Value)
	if err != nil {
		return
	}
	row, err := a.authStore.GetRefreshTokenByHash(r.Context(), hash)
	if err != nil {
		return
	}
	if rErr := a.authStore.RevokeRefreshToken(r.Context(), row.ID); rErr != nil {
		slog.Warn("logout: revoke failed", "token_id", row.ID, "err", rErr)
	}
}

func (a *API) issueSession(w http.ResponseWriter, r *http.Request, user queries.User, parentID *string) error {
	access, err := auth.IssueAccessToken(a.jwt, user.ID, user.IsAdmin)
	if err != nil {
		return err
	}
	refresh, err := auth.IssueRefreshToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := a.authStore.CreateRefreshToken(r.Context(), queries.CreateRefreshTokenParams{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: refresh.Hash,
		ExpiresAt: now.Add(auth.RefreshTTL).Format(time.RFC3339),
		ParentID:  parentID,
		CreatedAt: now.Format(time.RFC3339),
	}); err != nil {
		return err
	}
	setRefreshCookie(w, r, refresh.Plaintext)
	setAccessCookie(w, r, access)
	writeJSON(w, http.StatusOK, openapi.LoginResponse{
		AccessToken: access,
		ExpiresIn:   int(auth.AccessTTL.Seconds()),
	})
	return nil
}

func setRefreshCookie(w http.ResponseWriter, r *http.Request, plaintext string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure derived from request TLS state
		Name:     refreshCookieName,
		Value:    plaintext,
		Path:     refreshCookiePath,
		Expires:  time.Now().Add(auth.RefreshTTL),
		MaxAge:   int(auth.RefreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func setAccessCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // non-HttpOnly is required; browser img tags can't send headers
		Name:     "sarga_token",
		Value:    token,
		Path:     "/public/art",
		MaxAge:   int(auth.AccessTTL.Seconds()),
		HttpOnly: false,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure derived from request TLS state
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func parseRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

var dummyHash = mustDummyHash()

func mustDummyHash() string {
	h, err := auth.Hash("dummy-password-for-timing-purposes-only-not-a-secret")
	if err != nil {
		panic(err)
	}
	return h
}
