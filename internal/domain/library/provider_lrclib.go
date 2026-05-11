package library

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const lrclibBase = "https://lrclib.net/api"

var _ LyricsProvider = (*LRCLibClient)(nil)

type LRCLibClient struct {
	http *http.Client
}

func NewLRCLibClient() *LRCLibClient {
	return &LRCLibClient{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *LRCLibClient) Name() string { return "lrclib" }

// FetchLyrics tries an exact-match lookup first (/api/get), then falls back
// to a search (/api/search) selecting the result with the closest duration.
// durationSec should be the track duration in whole seconds (0 = unknown).
func (c *LRCLibClient) FetchLyrics(ctx context.Context, artist, title, album string, durationSec int) (*LyricsResult, error) {
	params := url.Values{
		"track_name":  {title},
		"artist_name": {artist},
	}
	if album != "" {
		params.Set("album_name", album)
	}
	if durationSec > 0 {
		params.Set("duration", strconv.Itoa(durationSec))
	}

	body, err := c.get(ctx, lrclibBase+"/get?"+params.Encode())
	if err != nil {
		return nil, err
	}
	if body != nil {
		return parseLRCLibResponse(body)
	}

	// 404 on /get → try search
	searchParams := url.Values{
		"track_name":  {title},
		"artist_name": {artist},
	}
	body, err = c.get(ctx, lrclibBase+"/search?"+searchParams.Encode())
	if err != nil || body == nil {
		return nil, err
	}
	var results []lrclibEntry
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("lrclib search: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	// Pick the result with the closest duration when we know ours
	best := results[0]
	if durationSec > 0 {
		bestDiff := abs(int(results[0].Duration) - durationSec)
		for _, r := range results[1:] {
			if d := abs(int(r.Duration) - durationSec); d < bestDiff {
				best, bestDiff = r, d
			}
		}
	}
	return entryToResult(best), nil
}

func (c *LRCLibClient) get(ctx context.Context, rawURL string) ([]byte, error) {
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lrclib: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 256<<10))
}

func parseLRCLibResponse(body []byte) (*LyricsResult, error) {
	var entry lrclibEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, fmt.Errorf("lrclib get: %w", err)
	}
	return entryToResult(entry), nil
}

func entryToResult(e lrclibEntry) *LyricsResult {
	if e.Instrumental || (e.PlainLyrics == "" && e.SyncedLyrics == "") {
		return nil
	}
	if e.SyncedLyrics != "" {
		return &LyricsResult{Content: e.SyncedLyrics, IsSynced: true, Source: "lrclib"}
	}
	return &LyricsResult{Content: e.PlainLyrics, IsSynced: false, Source: "lrclib"}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Response types ---------------------------------------------------------

type lrclibEntry struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}
