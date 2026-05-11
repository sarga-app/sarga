package api

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sarga-app/sarga/internal/transport/routes/api/gen"
)

// Catalog endpoints are wired directly via catalog.Mount, not through the
// gen.ServerInterface dispatch path. These stubs satisfy the compile-time check.

func (h *apiHandler) ListAlbums(w http.ResponseWriter, _ *http.Request, _ gen.ListAlbumsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) GetAlbum(w http.ResponseWriter, _ *http.Request, _ openapi_types.UUID, _ gen.GetAlbumParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) ListArtists(w http.ResponseWriter, _ *http.Request, _ gen.ListArtistsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) GetArtist(w http.ResponseWriter, _ *http.Request, _ openapi_types.UUID, _ gen.GetArtistParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) ListScanRuns(w http.ResponseWriter, _ *http.Request, _ openapi_types.UUID, _ gen.ListScanRunsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) ListPlaylists(w http.ResponseWriter, _ *http.Request, _ gen.ListPlaylistsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) GetPlaylist(w http.ResponseWriter, _ *http.Request, _ openapi_types.UUID, _ gen.GetPlaylistParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) Search(w http.ResponseWriter, _ *http.Request, _ gen.SearchParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *apiHandler) ListTracks(w http.ResponseWriter, _ *http.Request, _ gen.ListTracksParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
