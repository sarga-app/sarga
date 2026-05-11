package library

import "context"

// MatchResult is returned by an IdentificationProvider after matching a track
// against an external database.
type MatchResult struct {
	Confidence       float64
	Source           string // mbid_tag | mb_fuzzy | discogs_tag | discogs_fuzzy | acoustid | raw
	MBRecordingID    string
	MBReleaseID      string
	MBArtistID       string
	DiscogsReleaseID string
}

// ArtistMeta holds enrichment data fetched from artist metadata providers.
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

// AlbumMeta holds enrichment data fetched from album metadata providers.
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

// ArtworkResult describes a single piece of artwork from a provider.
type ArtworkResult struct {
	URL      string
	Source   string // caa | theaudiodb | fanart | lastfm | discogs | embedded | folder
	Width    int
	Height   int
	Bytes    []byte // non-nil when data is fetched inline rather than by URL
	MIMEType string
}

// LyricsResult holds lyrics returned by a lyrics provider.
type LyricsResult struct {
	Content  string
	IsSynced bool
	Language string
	Source   string // lrclib
}

// IdentificationProvider matches a track file to an external database entry.
type IdentificationProvider interface {
	Name() string
	Match(ctx context.Context, m Metadata) (*MatchResult, error)
}

// ArtistMetaProvider fetches artist biography, genres, and external IDs.
type ArtistMetaProvider interface {
	Name() string
	FetchArtist(ctx context.Context, mbid, name string) (*ArtistMeta, error)
}

// AlbumMetaProvider fetches album metadata from an external source.
type AlbumMetaProvider interface {
	Name() string
	FetchAlbum(ctx context.Context, mbid, artist, title string) (*AlbumMeta, error)
}

// ArtworkProvider fetches artwork for artists and albums.
type ArtworkProvider interface {
	Name() string
	FetchArtistArt(ctx context.Context, mbid string) ([]ArtworkResult, error)
	FetchAlbumArt(ctx context.Context, releaseMBID string) ([]ArtworkResult, error)
}

// LyricsProvider fetches synced or unsynced lyrics for a track.
type LyricsProvider interface {
	Name() string
	FetchLyrics(ctx context.Context, artist, title, album string, durationSec int) (*LyricsResult, error)
}
