package library

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// Fanart.tv uses the MusicBrainz artist MBID for artist images and the
// MusicBrainz Release Group MBID (not Release MBID) for album art.
// The enricher must pass release_groups.mbid, not releases.mbid.

const fanartBase = "https://webservice.fanart.tv/v3/music"

var _ ArtworkProvider = (*FanartClient)(nil)

type FanartClient struct {
	http       *http.Client
	limiter    *rate.Limiter
	projectKey string
}

func NewFanartClient(projectKey string) *FanartClient {
	return &FanartClient{
		http:       &http.Client{Timeout: 20 * time.Second},
		limiter:    rate.NewLimiter(rate.Every(time.Second/2), 2),
		projectKey: projectKey,
	}
}

func (c *FanartClient) Name() string { return "fanart" }

func (c *FanartClient) available() bool { return c.projectKey != "" }

// ArtworkProvider --------------------------------------------------------

// FetchArtistArt returns artist artwork using the MusicBrainz artist MBID.
func (c *FanartClient) FetchArtistArt(ctx context.Context, artistMBID string) ([]ArtworkResult, error) {
	if !c.available() || artistMBID == "" {
		return nil, nil
	}
	body, err := c.do(ctx, fmt.Sprintf("%s/%s?api_key=%s", fanartBase, artistMBID, c.projectKey))
	if err != nil || body == nil {
		return nil, err
	}

	var resp fanartArtistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fanart artist: %w", err)
	}

	var results []ArtworkResult
	// HD logo preferred over standard logo
	for _, img := range resp.HDMusicLogo {
		results = append(results, ArtworkResult{URL: img.URL, Source: "fanart", MIMEType: "image/png"})
	}
	for _, img := range resp.ArtistThumb {
		results = append(results, ArtworkResult{URL: img.URL, Source: "fanart", MIMEType: "image/jpeg"})
	}
	for _, img := range resp.ArtistBackground {
		results = append(results, ArtworkResult{URL: img.URL, Source: "fanart", MIMEType: "image/jpeg"})
	}
	return results, nil
}

// FetchAlbumArt returns album artwork using the MusicBrainz Release Group MBID.
// Important: pass release_groups.mbid, not releases.mbid.
func (c *FanartClient) FetchAlbumArt(ctx context.Context, releaseGroupMBID string) ([]ArtworkResult, error) {
	if !c.available() || releaseGroupMBID == "" {
		return nil, nil
	}
	// The artist endpoint embeds album art under the "albums" key, keyed by release group MBID.
	// We need to know the artist MBID to call the artist endpoint.
	// Without it, use the dedicated albums endpoint.
	body, err := c.do(ctx, fmt.Sprintf("%s/albums/%s?api_key=%s", fanartBase, releaseGroupMBID, c.projectKey))
	if err != nil || body == nil {
		return nil, err
	}

	var resp struct {
		Albums map[string]fanartAlbumImages `json:"albums"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fanart album: %w", err)
	}

	var results []ArtworkResult
	for _, album := range resp.Albums {
		for _, img := range album.AlbumCover {
			results = append(results, ArtworkResult{URL: img.URL, Source: "fanart", MIMEType: "image/jpeg"})
		}
		break // first entry is the one we want
	}
	return results, nil
}

// Internal helpers -------------------------------------------------------

func (c *FanartClient) do(ctx context.Context, rawURL string) ([]byte, error) {
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

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("fanart: rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fanart: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<10))
}

// Response types ---------------------------------------------------------

type fanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Likes string `json:"likes"`
}

type fanartArtistResponse struct {
	ArtistThumb      []fanartImage `json:"artistthumb"`
	ArtistBackground []fanartImage `json:"artistbackground"`
	MusicLogo        []fanartImage `json:"musiclogo"`
	HDMusicLogo      []fanartImage `json:"hdmusiclogo"`
	MusicBanner      []fanartImage `json:"musicbanner"`
}

type fanartAlbumImages struct {
	AlbumCover []fanartImage `json:"albumcover"`
	CDart      []fanartImage `json:"cdart"`
}
