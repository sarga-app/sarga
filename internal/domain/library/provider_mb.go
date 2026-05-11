package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	mbBase  = "https://musicbrainz.org/ws/2"
	caaBase = "https://coverartarchive.org"
)

var (
	_ IdentificationProvider = (*MusicBrainzClient)(nil)
	_ AlbumMetaProvider      = (*MusicBrainzClient)(nil)
	_ ArtworkProvider        = (*MusicBrainzClient)(nil)
)

type MusicBrainzClient struct {
	http    *http.Client
	limiter *rate.Limiter
	ua      string
}

func NewMusicBrainzClient(appVersion string) *MusicBrainzClient {
	return &MusicBrainzClient{
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Every(time.Second), 1),
		ua:      fmt.Sprintf("sarga/%s ( https://github.com/sarga-app/sarga )", appVersion),
	}
}

func (c *MusicBrainzClient) Name() string { return "musicbrainz" }

// IdentificationProvider -------------------------------------------------

func (c *MusicBrainzClient) Match(ctx context.Context, m Metadata) (*MatchResult, error) {
	if m.MBRecordingID != "" {
		rec, err := c.lookupRecording(ctx, m.MBRecordingID)
		if err != nil || rec == nil {
			return nil, err
		}
		return &MatchResult{
			Confidence:    1.0,
			Source:        "mbid_tag",
			MBRecordingID: rec.ID,
			MBReleaseID:   rec.bestReleaseID(),
			MBArtistID:    rec.primaryArtistID(),
		}, nil
	}

	if m.Title == "" {
		return nil, nil
	}
	recs, err := c.searchRecording(ctx, m.Title, m.PrimaryArtist())
	if err != nil || len(recs) == 0 {
		return nil, err
	}

	best, bestScore := recs[0], mbRecordingScore(m, recs[0])
	for _, rec := range recs[1:min(len(recs), 5)] {
		if s := mbRecordingScore(m, rec); s > bestScore {
			best, bestScore = rec, s
		}
	}
	if bestScore < 0.5 {
		return nil, nil
	}
	return &MatchResult{
		Confidence:    bestScore,
		Source:        "mb_fuzzy",
		MBRecordingID: best.ID,
		MBReleaseID:   best.bestReleaseID(),
		MBArtistID:    best.primaryArtistID(),
	}, nil
}

// AlbumMetaProvider ------------------------------------------------------

func (c *MusicBrainzClient) FetchAlbum(ctx context.Context, mbid, _, _ string) (*AlbumMeta, error) {
	if mbid == "" {
		return nil, nil
	}
	rel, err := c.lookupRelease(ctx, mbid)
	if err != nil || rel == nil {
		return nil, err
	}
	meta := &AlbumMeta{
		Title:   rel.Title,
		Date:    rel.Date,
		Country: rel.Country,
		MBID:    rel.ID,
	}
	if len(rel.LabelInfo) > 0 {
		meta.Label = rel.LabelInfo[0].Label.Name
		meta.CatalogNumber = rel.LabelInfo[0].CatalogNumber
	}
	return meta, nil
}

// ArtworkProvider (Cover Art Archive) ------------------------------------

func (c *MusicBrainzClient) FetchArtistArt(_ context.Context, _ string) ([]ArtworkResult, error) {
	return nil, nil // CAA provides release art only
}

func (c *MusicBrainzClient) FetchAlbumArt(ctx context.Context, releaseMBID string) ([]ArtworkResult, error) {
	if releaseMBID == "" {
		return nil, nil
	}
	rawURL := fmt.Sprintf("%s/release/%s/front-500", caaBase, releaseMBID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caa: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	mime := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	if mime == "" {
		mime = "image/jpeg"
	}
	return []ArtworkResult{{Source: "caa", Bytes: data, MIMEType: mime}}, nil
}

// Internal helpers -------------------------------------------------------

func (c *MusicBrainzClient) searchRecording(ctx context.Context, title, artist string) ([]mbRecording, error) {
	q := `recording:"` + luceneEscape(title) + `"`
	if artist != "" {
		q += ` AND artistname:"` + luceneEscape(artist) + `"`
	}
	params := url.Values{"query": {q}, "limit": {"5"}}
	body, err := c.do(ctx, mbBase+"/recording?"+params.Encode()+"&fmt=json")
	if err != nil || body == nil {
		return nil, err
	}
	var result struct {
		Recordings []mbRecording `json:"recordings"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("musicbrainz search: %w", err)
	}
	return result.Recordings, nil
}

func (c *MusicBrainzClient) lookupRecording(ctx context.Context, mbid string) (*mbRecording, error) {
	rawURL := fmt.Sprintf("%s/recording/%s?inc=artist-credits+releases&fmt=json", mbBase, mbid)
	body, err := c.do(ctx, rawURL)
	if err != nil || body == nil {
		return nil, err
	}
	var rec mbRecording
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, fmt.Errorf("musicbrainz lookup recording: %w", err)
	}
	return &rec, nil
}

func (c *MusicBrainzClient) lookupRelease(ctx context.Context, mbid string) (*mbReleaseDetail, error) {
	rawURL := fmt.Sprintf("%s/release/%s?inc=artist-credits+labels&fmt=json", mbBase, mbid)
	body, err := c.do(ctx, rawURL)
	if err != nil || body == nil {
		return nil, err
	}
	var rel mbReleaseDetail
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("musicbrainz lookup release: %w", err)
	}
	return &rel, nil
}

// do makes one rate-limited GET with exponential backoff on 503.
func (c *MusicBrainzClient) do(ctx context.Context, rawURL string) ([]byte, error) {
	backoff := time.Second
	for range 3 {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		body, status, headers, err := c.doOnce(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		switch status {
		case http.StatusOK:
			return body, nil
		case http.StatusNotFound:
			return nil, nil
		case http.StatusServiceUnavailable:
			wait := backoff
			if ra := headers.Get("Retry-After"); ra != "" {
				if secs, aerr := strconv.Atoi(ra); aerr == nil && secs > 0 {
					wait = time.Duration(secs) * time.Second
				}
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			backoff = min(backoff*2, 30*time.Second)
		default:
			return nil, fmt.Errorf("musicbrainz: HTTP %d", status)
		}
	}
	return nil, errors.New("musicbrainz: service unavailable after retries")
}

func (c *MusicBrainzClient) doOnce(ctx context.Context, rawURL string) (body []byte, status int, headers http.Header, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, resp.Header, err
}

// Response types ---------------------------------------------------------

type mbRecording struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Length       int                `json:"length"` // milliseconds
	ArtistCredit []mbArtistSlot     `json:"artist-credit"`
	Releases     []mbReleaseSummary `json:"releases"`
}

func (r mbRecording) bestReleaseID() string {
	for _, rel := range r.Releases {
		if rel.Status == "Official" {
			return rel.ID
		}
	}
	if len(r.Releases) > 0 {
		return r.Releases[0].ID
	}
	return ""
}

func (r mbRecording) primaryArtistID() string {
	if len(r.ArtistCredit) > 0 {
		return r.ArtistCredit[0].Artist.ID
	}
	return ""
}

func (r mbRecording) primaryArtistName() string {
	if len(r.ArtistCredit) > 0 {
		return r.ArtistCredit[0].Artist.Name
	}
	return ""
}

func (r mbRecording) bestReleaseTitle() string {
	for _, rel := range r.Releases {
		if rel.Status == "Official" {
			return rel.Title
		}
	}
	if len(r.Releases) > 0 {
		return r.Releases[0].Title
	}
	return ""
}

type mbArtistSlot struct {
	Name   string `json:"name"`
	Artist struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

type mbReleaseSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type mbReleaseDetail struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Country   string `json:"country"`
	LabelInfo []struct {
		Label struct {
			Name string `json:"name"`
		} `json:"label"`
		CatalogNumber string `json:"catalog-number"`
	} `json:"label-info"`
}

// Scoring ----------------------------------------------------------------

func mbRecordingScore(m Metadata, rec mbRecording) float64 {
	return mbFuzzyScore(m, rec.Title, rec.primaryArtistName(), rec.bestReleaseTitle(), 0, 0)
}

var luceneReplacer = strings.NewReplacer(
	`+`, `\+`, `-`, `\-`, `&`, `\&`, `|`, `\|`, `!`, `\!`,
	`(`, `\(`, `)`, `\)`, `{`, `\{`, `}`, `\}`,
	`[`, `\[`, `]`, `\]`, `^`, `\^`, `~`, `\~`,
	`*`, `\*`, `?`, `\?`, `:`, `\:`, `/`, `\/`,
	`"`, `\"`,
)

func luceneEscape(s string) string { return luceneReplacer.Replace(s) }
