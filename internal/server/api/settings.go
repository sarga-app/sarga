package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/db/queries"
)

const minCacheMB = 50

func (a *API) mountSettingsRoutes(r chi.Router) {
	r.Route("/server", func(r chi.Router) {
		r.Use(requireAuth(a.jwt))
		r.Get("/settings", a.getServerSettings)
		r.With(requireAdmin).Patch("/settings", a.updateServerSettings)
	})
}

func (a *API) getServerSettings(w http.ResponseWriter, r *http.Request) {
	q := queries.New(a.db.Read())
	s, err := q.GetServerSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to load settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"art_thumb_cache_mb":         s.ArtThumbCacheMb,
		"art_thumb_cache_used_bytes": a.thumbs.ThumbUsedBytes(),
		"lastfm_api_key_set":         len(s.LastfmApiKey) > 0,
		"lastfm_secret_set":          len(s.LastfmSecret) > 0,
		"fanart_api_key_set":         len(s.FanartApiKey) > 0,
		"acoustid_api_key_set":       len(s.AcoustidApiKey) > 0,
		"discogs_token_set":          len(s.DiscogsToken) > 0,
	})
}

type serverPatchBody struct {
	ArtThumbCacheMB *int    `json:"art_thumb_cache_mb"`
	LastFMAPIKey    *string `json:"lastfm_api_key"`
	LastFMSecret    *string `json:"lastfm_secret"`
	FanartAPIKey    *string `json:"fanart_api_key"`
	AcoustIDAPIKey  *string `json:"acoustid_api_key"`
	DiscogsToken    *string `json:"discogs_token"`
}

func (a *API) updateServerSettings(w http.ResponseWriter, r *http.Request) {
	var body serverPatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	ctx := r.Context()
	cur, err := queries.New(a.db.Read()).GetServerSettings(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to load settings")
		return
	}

	p := queries.UpdateServerSettingsParams{
		ArtThumbCacheMb: cur.ArtThumbCacheMb,
		LastfmApiKey:    cur.LastfmApiKey,
		LastfmSecret:    cur.LastfmSecret,
		FanartApiKey:    cur.FanartApiKey,
		AcoustidApiKey:  cur.AcoustidApiKey,
		DiscogsToken:    cur.DiscogsToken,
	}

	if body.ArtThumbCacheMB != nil {
		if *body.ArtThumbCacheMB < minCacheMB {
			writeError(w, http.StatusUnprocessableEntity, "invalid_value",
				"art_thumb_cache_mb must be >= 50")
			return
		}
		p.ArtThumbCacheMb = int64(*body.ArtThumbCacheMB)
		a.thumbs.SetCacheLimit(p.ArtThumbCacheMb)
	}

	for _, f := range []struct {
		in  *string
		dst *[]byte
	}{
		{body.LastFMAPIKey, &p.LastfmApiKey},
		{body.LastFMSecret, &p.LastfmSecret},
		{body.FanartAPIKey, &p.FanartApiKey},
		{body.AcoustIDAPIKey, &p.AcoustidApiKey},
		{body.DiscogsToken, &p.DiscogsToken},
	} {
		if f.in == nil {
			continue
		}
		sealed, err := a.cipher.Seal(*f.in)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encrypt_failed", "failed to seal secret")
			return
		}
		*f.dst = sealed
	}

	if _, err := queries.New(a.db.Write()).UpdateServerSettings(ctx, p); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to update settings")
		return
	}

	a.getServerSettings(w, r)
}
