package providers

import (
	"context"
	"math"
	"strings"

	"github.com/sarga-app/sarga/internal/tags"
)

type MatchResult struct {
	Confidence       float64
	Source           string // mbid_tag | mb_fuzzy | discogs_tag | discogs_fuzzy | acoustid | raw
	MBRecordingID    string
	MBReleaseID      string
	MBArtistID       string
	DiscogsReleaseID string
}

type ArtistMeta struct {
	Name      string
	SortName  string
	MBID      string
	Bio       string
	Country   string
	BeginYear int
	EndYear   int
	TADBID    string
	DiscogsID string
	Tags      []string
}

type AlbumMeta struct {
	Title         string
	Date          string
	Label         string
	CatalogNumber string
	Country       string
	MBID          string
	DiscogsID     string
	Tags          []string
}

type ArtworkResult struct {
	URL      string
	Source   string // caa | theaudiodb | fanart | lastfm | discogs | embedded | folder
	Width    int
	Height   int
	Bytes    []byte // non-nil when data is fetched inline rather than by URL
	MIMEType string
}

type LyricsResult struct {
	Content  string
	IsSynced bool
	Language string
	Source   string // lrclib
}

type IdentificationProvider interface {
	Name() string
	Match(ctx context.Context, m tags.Metadata) (*MatchResult, error)
}

type ArtistMetaProvider interface {
	Name() string
	FetchArtist(ctx context.Context, mbid, name string) (*ArtistMeta, error)
}

type AlbumMetaProvider interface {
	Name() string
	FetchAlbum(ctx context.Context, mbid, artist, title string) (*AlbumMeta, error)
}

type ArtworkProvider interface {
	Name() string
	FetchArtistArt(ctx context.Context, mbid string) ([]ArtworkResult, error)
	FetchAlbumArt(ctx context.Context, releaseMBID string) ([]ArtworkResult, error)
}

type LyricsProvider interface {
	Name() string
	FetchLyrics(ctx context.Context, artist, title, album string, durationSec int) (*LyricsResult, error)
}

func strSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return 1.0
	}
	maxLen := max(len([]rune(a)), len([]rune(b)))
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshtein([]rune(a), []rune(b))
	return 1.0 - float64(dist)/float64(maxLen)
}

func mbFuzzyScore(meta tags.Metadata, title, artist, album string, year int, durationMs int64) float64 {
	metaDurationMs := meta.DurationMs
	titleScore := strSimilarity(meta.Title, title) * 3.0
	artistScore := strSimilarity(meta.PrimaryArtist(), artist) * 3.0
	albumScore := strSimilarity(meta.Album, album) * 1.0
	total := titleScore + artistScore + albumScore

	if year > 0 && meta.Year > 0 {
		diff := math.Abs(float64(meta.Year - year))
		yearPenalty := math.Min(diff/10.0, 1.0) * 0.5
		total -= yearPenalty
	}

	if durationMs > 0 && metaDurationMs > 0 {
		diffSec := math.Abs(float64(metaDurationMs-durationMs)) / 1000.0
		durPenalty := math.Min(diffSec/30.0, 1.0)
		total -= durPenalty
	}

	return math.Max(0, total/7.5)
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	row := make([]int, len(b)+1)
	for j := range row {
		row[j] = j
	}
	for i, ca := range a {
		prev := i + 1
		for j, cb := range b {
			cost := 1
			if ca == cb {
				cost = 0
			}
			curr := min(min(prev+1, row[j+1]+1), row[j]+cost)
			row[j] = prev
			prev = curr
		}
		row[len(b)] = prev
	}
	return row[len(b)]
}
