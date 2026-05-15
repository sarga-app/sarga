package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/sarga-app/sarga/internal/tags"
)

const discogsBase = "https://api.discogs.com"

// Discogs ToS requires cache freshness ≤ 6 h for displayed content.
// Sarga stores enriched metadata in its own schema, not Discogs data verbatim,
// so this limit applies primarily to any future "view on Discogs" linking.

var (
	_ IdentificationProvider = (*DiscogsClient)(nil)
	_ AlbumMetaProvider      = (*DiscogsClient)(nil)
	_ ArtworkProvider        = (*DiscogsClient)(nil)
)

type DiscogsClient struct {
	http    *http.Client
	limiter *rate.Limiter
	token   string
	ua      string
}

func NewDiscogsClient(token, appVersion string) *DiscogsClient {
	return &DiscogsClient{
		http: &http.Client{Timeout: 30 * time.Second},
		// 55 req/min to stay comfortably under the 60/min authenticated limit.
		limiter: rate.NewLimiter(rate.Every(time.Minute/55), 5),
		token:   token,
		ua:      fmt.Sprintf("Sarga/%s +https://github.com/sarga-app/sarga", appVersion),
	}
}

func (c *DiscogsClient) Name() string { return "discogs" }

func (c *DiscogsClient) available() bool { return c.token != "" }

func (c *DiscogsClient) Match(ctx context.Context, m tags.Metadata) (*MatchResult, error) {
	if !c.available() || m.Title == "" {
		return nil, nil
	}
	// Fast-path: Discogs release ID already in tags
	if m.DiscogsReleaseID != "" {
		rel, err := c.getReleaseByID(ctx, m.DiscogsReleaseID)
		if err != nil || rel == nil {
			return nil, err
		}
		return &MatchResult{
			Confidence:       1.0,
			Source:           "discogs_tag",
			DiscogsReleaseID: rel.ID,
		}, nil
	}

	params := url.Values{
		"q":        {m.Title},
		"type":     {"release"},
		"per_page": {"5"},
	}
	if a := m.PrimaryArtist(); a != "" {
		params.Set("artist", a)
	}
	body, err := c.do(ctx, discogsBase+"/database/search?"+params.Encode())
	if err != nil || body == nil {
		return nil, err
	}
	var result struct {
		Results []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("discogs search: %w", err)
	}
	if len(result.Results) == 0 {
		return nil, nil
	}

	top := result.Results[0]
	// Discogs title format is "Artist - Album"; strip to get album name for scoring
	parts := strings.SplitN(top.Title, " - ", 2)
	candidate := top.Title
	if len(parts) == 2 {
		candidate = parts[1]
	}
	score := (strSimilarity(m.Title, candidate)*3 + strSimilarity(m.PrimaryArtist(), parts[0])*3) / 6
	if score < 0.5 {
		return nil, nil
	}
	return &MatchResult{
		Confidence:       score,
		Source:           "discogs_fuzzy",
		DiscogsReleaseID: fmt.Sprintf("%d", top.ID),
	}, nil
}

func (c *DiscogsClient) FetchAlbum(_ context.Context, _, _, _ string) (*AlbumMeta, error) {
	return nil, nil // Discogs album meta fetched only when DiscogsReleaseID is known (via enricher)
}

// FetchAlbumByDiscogsID fetches album metadata for a Discogs release ID.
// Called by the enricher when the match source is discogs_* and a release ID is known.
func (c *DiscogsClient) FetchAlbumByDiscogsID(ctx context.Context, discogsID string) (*AlbumMeta, error) {
	if !c.available() || discogsID == "" {
		return nil, nil
	}
	rel, err := c.getReleaseByID(ctx, discogsID)
	if err != nil || rel == nil {
		return nil, err
	}
	meta := &AlbumMeta{
		Title:     rel.Title,
		Date:      rel.Released,
		Country:   rel.Country,
		DiscogsID: rel.ID,
	}
	for _, lbl := range rel.Labels {
		meta.Label = lbl.Name
		meta.CatalogNumber = lbl.CatalogNumber
		break
	}
	meta.Tags = append(meta.Tags, rel.Genres...)
	meta.Tags = append(meta.Tags, rel.Styles...)
	return meta, nil
}

func (c *DiscogsClient) FetchArtistArt(_ context.Context, _ string) ([]ArtworkResult, error) {
	return nil, nil // Discogs artist images not used; TADB/Fanart.tv serve this
}

func (c *DiscogsClient) FetchAlbumArt(_ context.Context, _ string) ([]ArtworkResult, error) {
	return nil, nil // Discogs art requires the Discogs release ID, not an MBID; called via enricher
}

// FetchAlbumArtByDiscogsID fetches the primary cover art for a Discogs release.
func (c *DiscogsClient) FetchAlbumArtByDiscogsID(ctx context.Context, discogsID string) ([]ArtworkResult, error) {
	if !c.available() || discogsID == "" {
		return nil, nil
	}
	rel, err := c.getReleaseByID(ctx, discogsID)
	if err != nil || rel == nil {
		return nil, err
	}
	for _, img := range rel.Images {
		if img.Type == "primary" {
			return []ArtworkResult{{
				URL:    img.URI,
				Source: "discogs",
				Width:  img.Width,
				Height: img.Height,
			}}, nil
		}
	}
	if len(rel.Images) > 0 {
		img := rel.Images[0]
		return []ArtworkResult{{
			URL:    img.URI,
			Source: "discogs",
			Width:  img.Width,
			Height: img.Height,
		}}, nil
	}
	return nil, nil
}

func (c *DiscogsClient) getReleaseByID(ctx context.Context, id string) (*discogsRelease, error) {
	body, err := c.do(ctx, fmt.Sprintf("%s/releases/%s", discogsBase, id))
	if err != nil || body == nil {
		return nil, err
	}
	var rel discogsRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("discogs release: %w", err)
	}
	return &rel, nil
}

func (c *DiscogsClient) do(ctx context.Context, rawURL string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Discogs token="+c.token)
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("discogs: rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discogs: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

type discogsRelease struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Released string   `json:"released"`
	Country  string   `json:"country"`
	Genres   []string `json:"genres"`
	Styles   []string `json:"styles"`
	Labels   []struct {
		Name          string `json:"name"`
		CatalogNumber string `json:"catno"`
	} `json:"labels"`
	Images []struct {
		Type   string `json:"type"`
		URI    string `json:"uri"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"images"`
}
