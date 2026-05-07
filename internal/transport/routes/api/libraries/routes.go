package libraries

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/transport/httpx"
	"github.com/sarga-app/sarga/internal/transport/middleware"
)

const bodyLimit = 64 * 1024

func Mount(r chi.Router, h *Handler, jwt domauth.JWTConfig) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(jwt))
		r.Use(middleware.BodyLimit(bodyLimit))

		r.Get("/libraries", h.ListLibraries)
		r.Get("/libraries/{libraryId}", withLibraryID(h.GetLibrary))

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Post("/libraries", h.CreateLibrary)
			r.Put("/libraries/{libraryId}", withLibraryID(h.UpdateLibrary))
			r.Delete("/libraries/{libraryId}", withLibraryID(h.DeleteLibrary))
			r.Get("/libraries/{libraryId}/access", withLibraryID(h.ListLibraryMembers))
			r.Post("/libraries/{libraryId}/access", withLibraryID(h.GrantLibraryAccess))
			r.Delete("/libraries/{libraryId}/access/{userId}", withLibraryAndUserID(h.RevokeLibraryAccess))
		})
	})
}

type libraryHandler func(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID)

type libraryUserHandler func(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID, userID uuid.UUID)

func withLibraryID(fn libraryHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "libraryId"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "libraryId must be a valid UUID")
			return
		}
		fn(w, r, id)
	}
}

func withLibraryAndUserID(fn libraryUserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libID, err := uuid.Parse(chi.URLParam(r, "libraryId"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "libraryId must be a valid UUID")
			return
		}
		userID, err := uuid.Parse(chi.URLParam(r, "userId"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "userId must be a valid UUID")
			return
		}
		fn(w, r, libID, userID)
	}
}
