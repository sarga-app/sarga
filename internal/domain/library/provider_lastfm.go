package library

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
)

// Last.fm artist/album image URLs have been dead since 2023 — their image
// pipeline was shut down.  This client fetches bio text and genre tags only.
// ToS requires "powered by AudioScrobbler" attribution where bio text is shown.

const lastfmBase = "https://ws.audioscrobbler.com/2.0/"

var _ ArtistMetaProvider = (*LastFMClient)(nil)

type LastFMClient struct {
	http    *http.Client
	limiter *rate.Limiter
	apiKey  string
}

func NewLastFMClient(apiKey string) *LastFMClient {
	return &LastFMClient{
		http:    &http.Client{Timeout: 15 * time.Second},
		limiter: rate.NewLimiter(rate.Every(time.Second/3), 3),
		apiKey:  apiKey,
	}
}

func (c *LastFMClient) Name() string { return "lastfm" }

func (c *LastFMClient) available() bool { return c.apiKey != "" }

// ArtistMetaProvider -----------------------------------------------------

func (c *LastFMClient) FetchArtist(ctx context.Context, mbid, name string) (*ArtistMeta, error) {
	if !c.available() {
		return nil, nil
	}
	params := url.Values{
		"method":      {"artist.getinfo"},
		"api_key":     {c.apiKey},
		"format":      {"json"},
		"autocorrect": {"1"},
		"lang":        {"en"},
	}
	switch {
	case mbid != "":
		params.Set("mbid", mbid)
	case name != "":
		params.Set("artist", name)
	default:
		return nil, nil
	}

	body, err := c.do(ctx, lastfmBase+"?"+params.Encode())
	if err != nil || body == nil {
		return nil, err
	}

	var resp struct {
		Artist struct {
			Name string `json:"name"`
			MBID string `json:"mbid"`
			Tags struct {
				Tag []struct {
					Name string `json:"name"`
				} `json:"tag"`
			} `json:"tags"`
			Bio struct {
				Summary string `json:"summary"`
				Content string `json:"content"`
			} `json:"bio"`
		} `json:"artist"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("lastfm artist: %w", err)
	}
	if resp.Error != 0 {
		if resp.Error == 6 { // not found
			return nil, nil
		}
		return nil, fmt.Errorf("lastfm: error %d: %s", resp.Error, resp.Message)
	}

	meta := &ArtistMeta{
		Name: resp.Artist.Name,
		MBID: resp.Artist.MBID,
		Bio:  stripLastFMSuffix(resp.Artist.Bio.Content),
	}
	for _, tag := range resp.Artist.Tags.Tag {
		meta.Tags = append(meta.Tags, tag.Name)
	}
	return meta, nil
}

// Internal helpers -------------------------------------------------------

func (c *LastFMClient) do(ctx context.Context, rawURL string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sarga/dev ( https://github.com/sarga-app/sarga )")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("lastfm: rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lastfm: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<10))
}

// Last.fm appends a "<a href...>Read more on Last.fm</a>." suffix to bios.
func stripLastFMSuffix(bio string) string {
	if before, _, found := strings.Cut(bio, `<a href="https://www.last.fm`); found {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(bio)
}
