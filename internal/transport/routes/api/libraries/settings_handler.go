package libraries

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/httpx"
)

// GetLibrarySettings returns current per-library settings.
// If no settings row exists (pre-migration library), it is created with defaults first.
func (h *Handler) GetLibrarySettings(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(h.db.Read())
	s, err := q.GetLibrarySettings(ctx, libraryID.String())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to get settings")
			return
		}
		// Row missing — library predates the trigger. Create defaults.
		if _, execErr := h.db.Write().ExecContext(ctx,
			"INSERT OR IGNORE INTO library_settings (library_id) VALUES (?)",
			libraryID.String(),
		); execErr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to initialize settings")
			return
		}
		s, err = q.GetLibrarySettings(ctx, libraryID.String())
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to get settings after init")
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, settingsToMap(s))
}

// PutLibrarySettings replaces all per-library settings (admin only).
func (h *Handler) PutLibrarySettings(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	var body struct {
		ScanOnCreate             *bool    `json:"scan_on_create"`
		WatchForChanges          *bool    `json:"watch_for_changes"`
		ScanSchedule             *string  `json:"scan_schedule"`
		FollowSymlinks           *bool    `json:"follow_symlinks"`
		ScanPlaylistFiles        *bool    `json:"scan_playlist_files"`
		Extensions               []string `json:"extensions"`
		EnableMusicbrainz        *bool    `json:"enable_musicbrainz"`
		EnableAcoustid           *bool    `json:"enable_acoustid"`
		EnableDiscogs            *bool    `json:"enable_discogs"`
		MatchConfidenceThreshold *float64 `json:"match_confidence_threshold"`
		MetadataLanguage         *string  `json:"metadata_language"`
		AlbumArtPriority         []string `json:"album_art_priority"`
		ArtistImagePriority      []string `json:"artist_image_priority"`
		FormatPreference         []string `json:"format_preference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	ctx := r.Context()
	q := queries.New(h.db.Read())

	cur, err := q.GetLibrarySettings(ctx, libraryID.String())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to get settings")
			return
		}
		if _, execErr := h.db.Write().ExecContext(ctx,
			"INSERT OR IGNORE INTO library_settings (library_id) VALUES (?)",
			libraryID.String(),
		); execErr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to initialize settings")
			return
		}
		cur, err = q.GetLibrarySettings(ctx, libraryID.String())
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to get settings after init")
			return
		}
	}

	// Merge: only update fields that were provided in the body.
	p := queries.UpdateLibrarySettingsParams{
		ScanOnAdd:                 cur.ScanOnAdd,
		FsnotifyEnabled:           cur.FsnotifyEnabled,
		ScanSchedule:              cur.ScanSchedule,
		FollowSymlinks:            cur.FollowSymlinks,
		ScanPlaylistFiles:         cur.ScanPlaylistFiles,
		FileExtensions:            cur.FileExtensions,
		MbEnrichmentEnabled:       cur.MbEnrichmentEnabled,
		AcoustidEnabled:           cur.AcoustidEnabled,
		DiscogsEnabled:            cur.DiscogsEnabled,
		MatchThreshold:            cur.MatchThreshold,
		MetadataLanguage:          cur.MetadataLanguage,
		ArtistProviderPriority:    cur.ArtistProviderPriority,
		AlbumProviderPriority:     cur.AlbumProviderPriority,
		AlbumArtSourcePriority:    cur.AlbumArtSourcePriority,
		ArtistImageSourcePriority: cur.ArtistImageSourcePriority,
		FormatPreferenceOrder:     cur.FormatPreferenceOrder,
		LibraryID:                 libraryID.String(),
	}
	if body.ScanOnCreate != nil {
		p.ScanOnAdd = *body.ScanOnCreate
	}
	if body.WatchForChanges != nil {
		p.FsnotifyEnabled = *body.WatchForChanges
	}
	if body.ScanSchedule != nil {
		p.ScanSchedule = nilIfEmpty(*body.ScanSchedule)
	}
	if body.FollowSymlinks != nil {
		p.FollowSymlinks = *body.FollowSymlinks
	}
	if body.ScanPlaylistFiles != nil {
		p.ScanPlaylistFiles = *body.ScanPlaylistFiles
	}
	if body.Extensions != nil {
		p.FileExtensions = strings.Join(body.Extensions, ",")
	}
	if body.EnableMusicbrainz != nil {
		p.MbEnrichmentEnabled = *body.EnableMusicbrainz
	}
	if body.EnableAcoustid != nil {
		p.AcoustidEnabled = *body.EnableAcoustid
	}
	if body.EnableDiscogs != nil {
		p.DiscogsEnabled = *body.EnableDiscogs
	}
	if body.MatchConfidenceThreshold != nil {
		v := *body.MatchConfidenceThreshold
		if v < 0 || v > 1 {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_value", "match_confidence_threshold must be 0–1")
			return
		}
		p.MatchThreshold = v
	}
	if body.MetadataLanguage != nil {
		p.MetadataLanguage = *body.MetadataLanguage
	}
	if body.AlbumArtPriority != nil {
		p.AlbumArtSourcePriority = strings.Join(body.AlbumArtPriority, ",")
	}
	if body.ArtistImagePriority != nil {
		p.ArtistImageSourcePriority = strings.Join(body.ArtistImagePriority, ",")
	}
	if body.FormatPreference != nil {
		p.FormatPreferenceOrder = strings.Join(body.FormatPreference, ",")
	}

	wq := queries.New(h.db.Write())
	updated, err := wq.UpdateLibrarySettings(ctx, p)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to update settings")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, settingsToMap(updated))
}

func settingsToMap(s queries.LibrarySetting) map[string]any {
	return map[string]any{
		"scan_on_create":             s.ScanOnAdd,
		"watch_for_changes":          s.FsnotifyEnabled,
		"scan_schedule":              s.ScanSchedule,
		"follow_symlinks":            s.FollowSymlinks,
		"scan_playlist_files":        s.ScanPlaylistFiles,
		"extensions":                 splitCSV(s.FileExtensions),
		"enable_musicbrainz":         s.MbEnrichmentEnabled,
		"enable_acoustid":            s.AcoustidEnabled,
		"enable_discogs":             s.DiscogsEnabled,
		"match_confidence_threshold": s.MatchThreshold,
		"metadata_language":          s.MetadataLanguage,
		"album_art_priority":         splitCSV(s.AlbumArtSourcePriority),
		"artist_image_priority":      splitCSV(s.ArtistImageSourcePriority),
		"format_preference":          splitCSV(s.FormatPreferenceOrder),
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
