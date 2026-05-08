package libraries

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/httpx"
	"github.com/sarga-app/sarga/internal/transport/routes/api/gen"
)

type Handler struct {
	db *db.DB
}

func NewHandler(d *db.DB) *Handler {
	return &Handler{db: d}
}

func (h *Handler) ListLibraries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := queries.New(h.db.Read())

	var libs []queries.Library
	var err error
	if domauth.IsAdminFrom(ctx) {
		libs, err = q.ListLibraries(ctx)
	} else {
		uid, _ := domauth.UserIDFrom(ctx)
		libs, err = q.ListLibrariesForUser(ctx, uid.String())
	}
	if err != nil {
		slog.Error("list libraries", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	pathsByLib, err := pathsForLibraries(ctx, q)
	if err != nil {
		slog.Error("list libraries: load paths", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	out := make([]gen.Library, 0, len(libs))
	for _, l := range libs {
		out = append(out, toGenLibrary(l, pathsByLib[l.ID]))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"libraries": out})
}

func (h *Handler) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req gen.LibraryWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	cleanPaths, err := validateLibraryWrite(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx := r.Context()
	libID := uuid.NewString()

	lib, err := h.tx(ctx, func(q *queries.Queries) (queries.Library, error) {
		l, e := q.CreateLibrary(ctx, queries.CreateLibraryParams{
			ID:   libID,
			Name: strings.TrimSpace(req.Name),
		})
		if e != nil {
			return queries.Library{}, e
		}
		for _, p := range cleanPaths {
			if _, e := q.AddLibraryPath(ctx, queries.AddLibraryPathParams{
				ID:        uuid.NewString(),
				LibraryID: libID,
				Path:      p,
			}); e != nil {
				return queries.Library{}, e
			}
		}
		return l, nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "library name or path already exists")
			return
		}
		slog.Error("create library", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toGenLibrary(lib, cleanPaths))
}

func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(h.db.Read())

	lib, err := q.GetLibraryByID(ctx, libraryID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("get library", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if !domauth.IsAdminFrom(ctx) {
		uid, _ := domauth.UserIDFrom(ctx)
		if _, accessErr := q.GetLibraryAccess(ctx, queries.GetLibraryAccessParams{
			LibraryID: lib.ID,
			UserID:    uid.String(),
		}); accessErr != nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no access to this library")
			return
		}
	}

	paths, err := loadPaths(ctx, q, lib.ID)
	if err != nil {
		slog.Error("get library: load paths", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenLibrary(lib, paths))
}

func (h *Handler) UpdateLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	var req gen.LibraryWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	cleanPaths, err := validateLibraryWrite(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx := r.Context()
	lib, err := h.tx(ctx, func(q *queries.Queries) (queries.Library, error) {
		l, e := q.UpdateLibrary(ctx, queries.UpdateLibraryParams{
			ID:   libraryID.String(),
			Name: strings.TrimSpace(req.Name),
		})
		if e != nil {
			return queries.Library{}, e
		}
		if e := q.ClearLibraryPaths(ctx, l.ID); e != nil {
			return queries.Library{}, e
		}
		for _, p := range cleanPaths {
			if _, e := q.AddLibraryPath(ctx, queries.AddLibraryPathParams{
				ID:        uuid.NewString(),
				LibraryID: l.ID,
				Path:      p,
			}); e != nil {
				return queries.Library{}, e
			}
		}
		return l, nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		if isUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "library name or path already exists")
			return
		}
		slog.Error("update library", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toGenLibrary(lib, cleanPaths))
}

func (h *Handler) DeleteLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	q := queries.New(h.db.Write())
	if err := q.DeleteLibrary(r.Context(), libraryID.String()); err != nil {
		slog.Error("delete library", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListLibraryMembers(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(h.db.Read())
	if _, err := q.GetLibraryByID(ctx, libraryID.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("list library members: get library", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	rows, err := q.ListLibraryMembers(ctx, libraryID.String())
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
	ctx := r.Context()
	q := queries.New(h.db.Write())
	if _, err := q.GetLibraryByID(ctx, libraryID.String()); err != nil {
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
	if err := q.GrantLibraryAccess(ctx, queries.GrantLibraryAccessParams{
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

func (h *Handler) RevokeLibraryAccess(w http.ResponseWriter, r *http.Request, libraryID, userID uuid.UUID) {
	q := queries.New(h.db.Write())
	if err := q.RevokeLibraryAccess(r.Context(), queries.RevokeLibraryAccessParams{
		LibraryID: libraryID.String(),
		UserID:    userID.String(),
	}); err != nil {
		slog.Error("revoke library access", "library_id", libraryID, "user_id", userID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tx runs fn inside a write transaction. fn returning an error rolls back.
func (h *Handler) tx(ctx context.Context, fn func(*queries.Queries) (queries.Library, error)) (queries.Library, error) {
	tx, err := h.db.Write().BeginTx(ctx, nil)
	if err != nil {
		return queries.Library{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	lib, err := fn(queries.New(tx))
	if err != nil {
		return queries.Library{}, err
	}
	if err := tx.Commit(); err != nil {
		return queries.Library{}, fmt.Errorf("commit: %w", err)
	}
	return lib, nil
}

func loadPaths(ctx context.Context, q *queries.Queries, libraryID string) ([]string, error) {
	rows, err := q.ListLibraryPaths(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(rows))
	for _, r := range rows {
		paths = append(paths, r.Path)
	}
	return paths, nil
}

func pathsForLibraries(ctx context.Context, q *queries.Queries) (map[string][]string, error) {
	all, err := q.ListAllLibraryPaths(ctx)
	if err != nil {
		return nil, err
	}
	byLib := make(map[string][]string, len(all))
	for _, p := range all {
		byLib[p.LibraryID] = append(byLib[p.LibraryID], p.Path)
	}
	return byLib, nil
}

func validateLibraryWrite(req gen.LibraryWrite) ([]string, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name must not be empty")
	}
	if len(req.Paths) == 0 {
		return nil, errors.New("at least one path is required")
	}
	seen := make(map[string]struct{}, len(req.Paths))
	out := make([]string, 0, len(req.Paths))
	for _, p := range req.Paths {
		clean := filepath.Clean(strings.TrimSpace(p))
		if clean == "" || clean == "." {
			return nil, errors.New("paths must not be empty")
		}
		if _, dup := seen[clean]; dup {
			return nil, fmt.Errorf("duplicate path %q", clean)
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func toGenLibrary(l queries.Library, paths []string) gen.Library {
	if paths == nil {
		paths = []string{}
	}
	return gen.Library{
		Id:        uuid.MustParse(l.ID),
		Name:      l.Name,
		Paths:     paths,
		CreatedAt: parseTime(l.CreatedAt),
		UpdatedAt: parseTime(l.UpdatedAt),
	}
}

// parseTime returns the parsed RFC3339 time, or the zero time on a parse
// failure (treats "couldn't read it" as "earliest possible" so any time-
// based gating downstream fires rather than silently passes).
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
