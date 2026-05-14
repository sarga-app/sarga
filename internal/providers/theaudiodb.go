package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

// TheAudioDB free key "2" is restricted for production use.
// A Patreon subscription ($8/mo) provides a private key with 100 req/min.
// Without a configured API key this provider is silently disabled.

var (
	_ ArtistMetaProvider = (*TheAudioDBClient)(nil)
	_ AlbumMetaProvider  = (*TheAudioDBClient)(nil)
	_ ArtworkProvider    = (*TheAudioDBClient)(nil)
)

type TheAudioDBClient struct {
	http    *http.Client
	limiter *rate.Limiter
	apiKey  string
}

func NewTheAudioDBClient(apiKey string) *TheAudioDBClient {
	return &TheAudioDBClient{
		http:    &http.Client{Timeout: 20 * time.Second},
		limiter: rate.NewLimiter(rate.Every(time.Minute/25), 3),
		apiKey:  apiKey,
	}
}

func (c *TheAudioDBClient) Name() string { return "theaudiodb" }

func (c *TheAudioDBClient) available() bool { return c.apiKey != "" }

func (c *TheAudioDBClient) base() string {
	return "https://www.theaudiodb.com/api/v1/json/" + url.PathEscape(c.apiKey)
}

func (c *TheAudioDBClient) FetchArtist(ctx context.Context, mbid, name string) (*ArtistMeta, error) {
	if !c.available() {
		return nil, nil
	}
	var endpoint string
	switch {
	case mbid != "":
		endpoint = c.base() + "/artist-mb.php?i=" + url.QueryEscape(mbid)
	case name != "":
		endpoint = c.base() + "/search.php?s=" + url.QueryEscape(name)
	default:
		return nil, nil
	}

	body, err := c.do(ctx, endpoint)
	if err != nil || body == nil {
		return nil, err
	}
	var resp struct {
		Artists []tadbArtist `json:"artists"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("theaudiodb artist: %w", err)
	}
	if len(resp.Artists) == 0 {
		return nil, nil
	}
	a := resp.Artists[0]
	meta := &ArtistMeta{
		Name:    a.StrArtist,
		Country: a.StrCountry,
		Bio:     a.StrBiographyEN,
		TADBID:  a.IDArtist,
	}
	if a.IntFormedYear != "" {
		meta.BeginYear = atoi(a.IntFormedYear)
	}
	if a.StrGenre != "" {
		meta.Tags = []string{a.StrGenre}
	}
	return meta, nil
}

func (c *TheAudioDBClient) FetchAlbum(ctx context.Context, mbid, artist, title string) (*AlbumMeta, error) {
	if !c.available() {
		return nil, nil
	}
	// mbid here is the MB Release Group MBID; TADB uses release groups not releases
	var endpoint string
	switch {
	case mbid != "":
		endpoint = c.base() + "/album-mb.php?i=" + url.QueryEscape(mbid)
	case artist != "" && title != "":
		endpoint = c.base() + "/searchalbum.php?s=" + url.QueryEscape(artist) + "&a=" + url.QueryEscape(title)
	default:
		return nil, nil
	}

	body, err := c.do(ctx, endpoint)
	if err != nil || body == nil {
		return nil, err
	}
	var resp struct {
		Album []tadbAlbum `json:"album"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("theaudiodb album: %w", err)
	}
	if len(resp.Album) == 0 {
		return nil, nil
	}
	al := resp.Album[0]
	meta := &AlbumMeta{
		Title: al.StrAlbum,
		Label: al.StrLabel,
	}
	if al.IntYearReleased != "" {
		meta.Date = al.IntYearReleased
	}
	if al.StrGenre != "" {
		meta.Tags = []string{al.StrGenre}
	}
	return meta, nil
}

func (c *TheAudioDBClient) FetchArtistArt(ctx context.Context, mbid string) ([]ArtworkResult, error) {
	if !c.available() || mbid == "" {
		return nil, nil
	}
	body, err := c.do(ctx, c.base()+"/artist-mb.php?i="+url.QueryEscape(mbid))
	if err != nil || body == nil {
		return nil, err
	}
	var resp struct {
		Artists []tadbArtist `json:"artists"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Artists) == 0 {
		return nil, err
	}
	a := resp.Artists[0]
	var results []ArtworkResult
	if a.StrArtistThumb != "" {
		results = append(results, ArtworkResult{URL: a.StrArtistThumb, Source: "theaudiodb", MIMEType: "image/jpeg"})
	}
	for _, bg := range []string{a.StrArtistFanart, a.StrArtistFanart2, a.StrArtistFanart3} {
		if bg != "" {
			results = append(results, ArtworkResult{URL: bg, Source: "theaudiodb", MIMEType: "image/jpeg"})
		}
	}
	return results, nil
}

func (c *TheAudioDBClient) FetchAlbumArt(ctx context.Context, releaseGroupMBID string) ([]ArtworkResult, error) {
	if !c.available() || releaseGroupMBID == "" {
		return nil, nil
	}
	body, err := c.do(ctx, c.base()+"/album-mb.php?i="+url.QueryEscape(releaseGroupMBID))
	if err != nil || body == nil {
		return nil, err
	}
	var resp struct {
		Album []tadbAlbum `json:"album"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Album) == 0 {
		return nil, err
	}
	al := resp.Album[0]
	var results []ArtworkResult
	if al.StrAlbumThumbHQ != "" {
		results = append(results, ArtworkResult{URL: al.StrAlbumThumbHQ, Source: "theaudiodb", MIMEType: "image/jpeg"})
	} else if al.StrAlbumThumb != "" {
		results = append(results, ArtworkResult{URL: al.StrAlbumThumb, Source: "theaudiodb", MIMEType: "image/jpeg"})
	}
	return results, nil
}

func (c *TheAudioDBClient) do(ctx context.Context, rawURL string) ([]byte, error) {
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
		return nil, fmt.Errorf("theaudiodb: rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("theaudiodb: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}

type tadbArtist struct {
	IDArtist         string `json:"idArtist"`
	StrArtist        string `json:"strArtist"`
	StrCountry       string `json:"strCountry"`
	StrBiographyEN   string `json:"strBiographyEN"`
	StrGenre         string `json:"strGenre"`
	IntFormedYear    string `json:"intFormedYear"`
	StrArtistThumb   string `json:"strArtistThumb"`
	StrArtistFanart  string `json:"strArtistFanart"`
	StrArtistFanart2 string `json:"strArtistFanart2"`
	StrArtistFanart3 string `json:"strArtistFanart3"`
}

type tadbAlbum struct {
	IDAlbum         string `json:"idAlbum"`
	StrAlbum        string `json:"strAlbum"`
	IntYearReleased string `json:"intYearReleased"`
	StrLabel        string `json:"strLabel"`
	StrGenre        string `json:"strGenre"`
	StrAlbumThumb   string `json:"strAlbumThumb"`
	StrAlbumThumbHQ string `json:"strAlbumThumbHQ"`
}

func atoi(s string) int {
	var n int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		}
	}
	return n
}
