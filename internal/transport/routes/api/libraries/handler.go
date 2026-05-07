package libraries

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/httpx"
	"github.com/sarga-app/sarga/internal/transport/routes/api/gen"
)

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ListLibraries(w http.ResponseWriter, r *http.Request) {
	var libs []queries.Library
	var err error
	if domauth.IsAdminFrom(r.Context()) {
		libs, err = h.store.ListLibraries(r.Context())
	} else {
		uid, _ := domauth.UserIDFrom(r.Context())
		libs, err = h.store.ListLibrariesForUser(r.Context(), uid.String())
	}
	if err != nil {
		slog.Error("list libraries", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	out := make([]gen.Library, 0, len(libs))
	for _, l := range libs {
		out = append(out, toGenLibrary(l))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"libraries": out})
}

func (h *Handler) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req gen.LibraryWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if err := validateLibraryWrite(req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	lib, err := h.store.CreateLibrary(r.Context(), queries.CreateLibraryParams{
		ID:   uuid.NewString(),
		Name: strings.TrimSpace(req.Name),
		Path: filepath.Clean(req.Path),
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "library name already exists")
			return
		}
		slog.Error("create library", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toGenLibrary(lib))
}

func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	lib, err := h.store.GetLibraryByID(r.Context(), libraryID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("get library", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if !domauth.IsAdminFrom(r.Context()) {
		uid, _ := domauth.UserIDFrom(r.Context())
		if !h.userHasAccess(r, lib.ID, uid.String()) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no access to this library")
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, toGenLibrary(lib))
}

func (h *Handler) UpdateLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	var req gen.LibraryWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if err := validateLibraryWrite(req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	lib, err := h.store.UpdateLibrary(r.Context(), queries.UpdateLibraryParams{
		Name: strings.TrimSpace(req.Name),
		Path: filepath.Clean(req.Path),
		ID:   libraryID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		if isUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "library name already exists")
			return
		}
		slog.Error("update library", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenLibrary(lib))
}

func (h *Handler) DeleteLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	if err := h.store.DeleteLibrary(r.Context(), libraryID.String()); err != nil {
		slog.Error("delete library", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListLibraryMembers(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	if _, err := h.store.GetLibraryByID(r.Context(), libraryID.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("list library members: get library", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	rows, err := h.store.ListLibraryMembers(r.Context(), libraryID.String())
	if err != nil {
		slog.Error("list library members", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	out := make([]gen.LibraryMember, 0, len(rows))
	for _, m := range rows {
		member := gen.LibraryMember{
			UserId:    uuid.MustParse(m.UserID),
			Username:  m.Username,
			Role:      gen.LibraryMemberRole(m.Role),
			GrantedAt: parseTime(m.GrantedAt),
		}
		member.DisplayName = m.DisplayName
		out = append(out, member)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"members": out})
}

func (h *Handler) GrantLibraryAccess(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	if _, err := h.store.GetLibraryByID(r.Context(), libraryID.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("grant library access: get library", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	var req gen.GrantAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if req.UserId == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "user_id is required")
		return
	}
	role := string(req.Role)
	if role != "viewer" && role != "contributor" && role != "admin" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "role must be viewer, contributor, or admin")
		return
	}
	if err := h.store.GrantLibraryAccess(r.Context(), queries.GrantLibraryAccessParams{
		LibraryID: libraryID.String(),
		UserID:    req.UserId.String(),
		Role:      role,
	}); err != nil {
		slog.Error("grant library access", "library_id", libraryID, "user_id", req.UserId, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RevokeLibraryAccess(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID, userID uuid.UUID) {
	if err := h.store.RevokeLibraryAccess(r.Context(), queries.RevokeLibraryAccessParams{
		LibraryID: libraryID.String(),
		UserID:    userID.String(),
	}); err != nil {
		slog.Error("revoke library access", "library_id", libraryID, "user_id", userID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) userHasAccess(r *http.Request, libraryID, userID string) bool {
	_, err := h.store.GetLibraryAccess(r.Context(), queries.GetLibraryAccessParams{
		LibraryID: libraryID,
		UserID:    userID,
	})
	return err == nil
}

func validateLibraryWrite(req gen.LibraryWrite) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name must not be empty")
	}
	if strings.TrimSpace(req.Path) == "" {
		return errors.New("path must not be empty")
	}
	return nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func toGenLibrary(l queries.Library) gen.Library {
	return gen.Library{
		Id:        uuid.MustParse(l.ID),
		Name:      l.Name,
		Path:      l.Path,
		CreatedAt: parseTime(l.CreatedAt),
		UpdatedAt: parseTime(l.UpdatedAt),
	}
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
