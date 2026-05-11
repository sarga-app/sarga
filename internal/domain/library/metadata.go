package library

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

type Metadata struct {
	Title       string
	SortTitle   string
	Artists     []string
	AlbumArtist string
	Album       string
	SortAlbum   string
	Composer    string
	Genre       []string
	Year        int
	TrackNum    int
	TrackTotal  int
	DiscNum     int
	DiscTotal   int

	MBRecordingID    string
	MBReleaseID      string
	MBReleaseTrackID string
	MBArtistID       string
	DiscogsReleaseID string

	ISRC string

	BPM    int
	KeySig string

	RGTrackGain float64
	RGTrackPeak float64
	RGAlbumGain float64
	RGAlbumPeak float64

	Lyrics string

	EmbeddedArt     []byte
	EmbeddedArtMIME string

	DurationMs int64

	Format  string
	Codec   string
	TagFmt  tag.Format
	FileTyp tag.FileType
}

func Extract(path string) (Metadata, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from scanner's validated walk
	if err != nil {
		return Metadata{}, err
	}
	defer func() { _ = f.Close() }()

	m, err := tag.ReadFrom(f)
	if err != nil {
		if errors.Is(err, tag.ErrNoTagsFound) || strings.Contains(err.Error(), "no tags") {
			if _, ft, ferr := tag.Identify(f); ferr == nil {
				fm, cd := fileTypeToFormatCodec(ft)
				return Metadata{Format: fm, Codec: cd, FileTyp: ft}, nil
			}
			return Metadata{}, nil
		}
		return Metadata{}, fmt.Errorf("read tags %q: %w", path, err)
	}

	raw := m.Raw()
	trackNum, trackTotal := m.Track()
	discNum, discTotal := m.Disc()

	md := Metadata{
		Title:       m.Title(),
		AlbumArtist: m.AlbumArtist(),
		Album:       m.Album(),
		Composer:    m.Composer(),
		Year:        m.Year(),
		TrackNum:    trackNum,
		TrackTotal:  trackTotal,
		DiscNum:     discNum,
		DiscTotal:   discTotal,
		Lyrics:      m.Lyrics(),
		TagFmt:      m.Format(),
		FileTyp:     m.FileType(),
	}

	md.Format, md.Codec = fileTypeToFormatCodec(m.FileType())

	md.Artists = rawStrings(raw, "ARTISTS", "artists", "TPE1", "©ART")
	if len(md.Artists) == 0 && m.Artist() != "" {
		md.Artists = []string{m.Artist()}
	}

	md.SortTitle = rawString(raw, "TITLESORT", "TSOT", "soal")
	md.SortAlbum = rawString(raw, "ALBUMSORT", "TSOA", "soaa")

	if genres := rawStrings(raw, "GENRE", "genre", "©gen", "TCON"); len(genres) > 0 {
		md.Genre = genres
	} else if g := m.Genre(); g != "" {
		md.Genre = []string{g}
	}

	md.MBRecordingID = rawString(
		raw,
		"MUSICBRAINZ_TRACKID",
		"MusicBrainz Track Id", "MusicBrainz/Track Id",
		"TXXX:MusicBrainz Track Id",
		"----:com.apple.iTunes:MusicBrainz Track Id",
	)
	md.MBReleaseID = rawString(
		raw,
		"MUSICBRAINZ_ALBUMID",
		"MusicBrainz Album Id", "MusicBrainz/Album Id",
		"TXXX:MusicBrainz Album Id",
		"----:com.apple.iTunes:MusicBrainz Album Id",
	)
	md.MBReleaseTrackID = rawString(
		raw,
		"MUSICBRAINZ_RELEASETRACKID",
		"MusicBrainz Release Track Id",
		"TXXX:MusicBrainz Release Track Id",
		"----:com.apple.iTunes:MusicBrainz Release Track Id",
	)
	md.MBArtistID = rawString(
		raw,
		"MUSICBRAINZ_ARTISTID",
		"MusicBrainz Artist Id",
		"TXXX:MusicBrainz Artist Id",
		"----:com.apple.iTunes:MusicBrainz Artist Id",
	)

	md.DiscogsReleaseID = rawString(
		raw,
		"DISCOGS_RELEASE_ID",
		"Discogs_Release_Id",
		"TXXX:DISCOGS_RELEASE_ID",
		"----:com.apple.iTunes:DISCOGS_RELEASE_ID",
	)

	md.ISRC = rawString(raw, "ISRC", "TSRC", "----:com.apple.iTunes:ISRC")

	if bpmRaw, ok := rawAny(raw, "BPM", "TBPM", "tmpo", "BEATS PER MINUTE"); ok {
		md.BPM = parseIntVal(bpmRaw)
	}

	md.KeySig = rawString(raw, "INITIALKEY", "KEY", "TKEY", "----:com.apple.iTunes:initialkey")

	md.RGTrackGain = parseReplayGain(rawString(
		raw,
		"REPLAYGAIN_TRACK_GAIN",
		"TXXX:REPLAYGAIN_TRACK_GAIN",
		"----:com.apple.iTunes:REPLAYGAIN_TRACK_GAIN",
	))
	md.RGTrackPeak = parseReplayGain(rawString(
		raw,
		"REPLAYGAIN_TRACK_PEAK",
		"TXXX:REPLAYGAIN_TRACK_PEAK",
		"----:com.apple.iTunes:REPLAYGAIN_TRACK_PEAK",
	))
	md.RGAlbumGain = parseReplayGain(rawString(
		raw,
		"REPLAYGAIN_ALBUM_GAIN",
		"TXXX:REPLAYGAIN_ALBUM_GAIN",
		"----:com.apple.iTunes:REPLAYGAIN_ALBUM_GAIN",
	))
	md.RGAlbumPeak = parseReplayGain(rawString(
		raw,
		"REPLAYGAIN_ALBUM_PEAK",
		"TXXX:REPLAYGAIN_ALBUM_PEAK",
		"----:com.apple.iTunes:REPLAYGAIN_ALBUM_PEAK",
	))

	if v, ok := rawAny(raw, "TLEN"); ok {
		if ms := parseIntVal(v); ms > 0 {
			md.DurationMs = int64(ms)
		}
	}
	if md.DurationMs == 0 {
		md.DurationMs = parseVorbisDuration(rawString(raw, "DURATION", "duration"))
	}
	if md.DurationMs == 0 {
		if v, ok := rawAny(raw, "LENGTH", "length"); ok {
			if ms := parseIntVal(v); ms > 0 {
				md.DurationMs = int64(ms)
			}
		}
	}

	if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
		md.EmbeddedArt = pic.Data
		md.EmbeddedArtMIME = pic.MIMEType
	}

	return md, nil
}

func (m Metadata) HasMBID() bool { return m.MBRecordingID != "" }

func (m Metadata) PrimaryArtist() string {
	if len(m.Artists) == 0 {
		return ""
	}
	return m.Artists[0]
}

func fileTypeToFormatCodec(ft tag.FileType) (format, codec string) {
	switch ft {
	case tag.FLAC:
		return "flac", "FLAC"
	case tag.MP3:
		return "mp3", "MP3"
	case tag.M4A:
		return "m4a", "AAC"
	case tag.M4B:
		return "m4b", "AAC"
	case tag.M4P:
		return "m4p", "AAC"
	case tag.ALAC:
		return "m4a", "ALAC"
	case tag.OGG:
		return "ogg", "VORBIS"
	case tag.DSF:
		return "dsf", "DSD"
	case tag.UnknownFileType:
		return "unknown", "UNKNOWN"
	}
	s := strings.ToLower(string(ft))
	return s, strings.ToUpper(s)
}

func rawString(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		if s := anyToString(v); s != "" {
			return s
		}
	}
	return ""
}

func rawStrings(raw map[string]any, keys ...string) []string {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case []string:
			if len(t) > 0 {
				return t
			}
		case string:
			if t != "" {
				return []string{t}
			}
		}
	}
	return nil
}

func rawAny(raw map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			return v, true
		}
	}
	return nil, false
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []string:
		if len(t) > 0 {
			return t[0]
		}
	case fmt.Stringer:
		return t.String()
	case []byte:
		return string(t)
	}
	return ""
}

func parseIntVal(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

// parseVorbisDuration handles Vorbis DURATION tags, which should be seconds but
// some encoders (Spotify-derived FLACs) write milliseconds instead.
// Anything over 24 h in seconds must already be in ms.
func parseVorbisDuration(s string) int64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f <= 0 {
		return 0
	}
	const maxReasonableSeconds = float64(24 * 60 * 60)
	if f > maxReasonableSeconds {
		return int64(f)
	}
	return int64(f * 1000)
}

func parseReplayGain(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " dB")
	s = strings.TrimSuffix(s, "dB")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
