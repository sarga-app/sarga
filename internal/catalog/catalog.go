package catalog

import (
	"path/filepath"
	"strconv"
	"strings"

	"go.senan.xyz/taglib"
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

	DurationMs    int64
	Bitrate       int
	SampleRate    int
	Channels      int
	BitsPerSample int

	Format string
	Codec  string
}

func Extract(path string) (Metadata, error) {
	f, err := taglib.OpenReadOnly(path, taglib.WithReadStyle(taglib.ReadStyleAverage))
	if err != nil {
		return Metadata{Format: formatFromExt(path)}, nil
	}
	defer func() { _ = f.Close() }()

	all := f.AllTags()
	props := f.Properties()

	tags := normalizeTags(all.Tags)
	raw := normalizeTags(all.Raw)

	md := Metadata{
		DurationMs:    props.Length.Milliseconds(),
		Bitrate:       int(props.Bitrate),
		SampleRate:    int(props.SampleRate),
		Channels:      int(props.Channels),
		BitsPerSample: int(props.BitsPerSample),
		Codec:         props.Codec,
		Format:        formatFromExt(path),
	}
	if md.Format == "" {
		md.Format = strings.ToLower(props.Codec)
	}

	md.Title = pickString(tags, raw, "TITLE")
	md.Album = pickString(tags, raw, "ALBUM")
	md.AlbumArtist = pickString(tags, raw, "ALBUMARTIST")
	md.Composer = pickString(tags, raw, "COMPOSER")

	md.Artists = pickStrings(tags, raw, "ARTISTS", "ARTIST")
	if len(md.Artists) == 0 {
		if a := pickString(tags, raw, "ARTIST"); a != "" {
			md.Artists = []string{a}
		}
	}

	md.SortTitle = pickString(tags, raw, "TITLESORT")
	md.SortAlbum = pickString(tags, raw, "ALBUMSORT")

	md.Genre = pickStrings(tags, raw, "GENRE")

	md.Year = parseYear(pickString(tags, raw, "DATE", "YEAR", "ORIGINALDATE"))
	md.TrackNum, md.TrackTotal = parseTuple(pickString(tags, raw, "TRACKNUMBER"), pickString(tags, raw, "TRACKTOTAL"))
	md.DiscNum, md.DiscTotal = parseTuple(pickString(tags, raw, "DISCNUMBER"), pickString(tags, raw, "DISCTOTAL"))

	md.MBRecordingID = pickString(tags, raw, "MUSICBRAINZ_TRACKID")
	md.MBReleaseID = pickString(tags, raw, "MUSICBRAINZ_ALBUMID")
	md.MBReleaseTrackID = pickString(tags, raw, "MUSICBRAINZ_RELEASETRACKID")
	md.MBArtistID = pickString(tags, raw, "MUSICBRAINZ_ARTISTID")

	md.DiscogsReleaseID = pickString(tags, raw, "DISCOGS_RELEASE_ID")

	md.ISRC = pickString(tags, raw, "ISRC")

	md.BPM = atoi(pickString(tags, raw, "BPM", "TBPM", "BEATS PER MINUTE", "TEMPO"))

	md.KeySig = pickString(tags, raw, "INITIALKEY", "KEY", "TKEY")

	md.RGTrackGain = parseReplayGain(pickString(tags, raw, "REPLAYGAIN_TRACK_GAIN"))
	md.RGTrackPeak = parseReplayGain(pickString(tags, raw, "REPLAYGAIN_TRACK_PEAK"))
	md.RGAlbumGain = parseReplayGain(pickString(tags, raw, "REPLAYGAIN_ALBUM_GAIN"))
	md.RGAlbumPeak = parseReplayGain(pickString(tags, raw, "REPLAYGAIN_ALBUM_PEAK"))

	md.Lyrics = pickString(tags, raw, "LYRICS", "UNSYNCEDLYRICS")

	if len(props.Images) > 0 {
		if img, err := f.Image(0); err == nil && len(img) > 0 {
			md.EmbeddedArt = img
			md.EmbeddedArtMIME = props.Images[0].MIMEType
		}
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

func normalizeTags(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[strings.ToUpper(k)] = v
	}
	return out
}

func pickString(primary, fallback map[string][]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := primary[k]; ok && len(v) > 0 && v[0] != "" {
			return v[0]
		}
	}
	for _, k := range keys {
		for rk, rv := range fallback {
			if len(rv) == 0 || rv[0] == "" {
				continue
			}
			if rk == k || strings.HasSuffix(rk, ":"+k) {
				return rv[0]
			}
		}
	}
	return ""
}

func pickStrings(primary, fallback map[string][]string, keys ...string) []string {
	for _, k := range keys {
		if v, ok := primary[k]; ok && len(v) > 0 {
			return v
		}
	}
	for _, k := range keys {
		for rk, rv := range fallback {
			if len(rv) == 0 {
				continue
			}
			if rk == k || strings.HasSuffix(rk, ":"+k) {
				return rv
			}
		}
	}
	return nil
}

func parseTuple(num, total string) (int, int) {
	n := 0
	t := atoi(total)
	num = strings.TrimSpace(num)
	if i := strings.IndexByte(num, '/'); i >= 0 {
		n = atoi(num[:i])
		if t == 0 {
			t = atoi(num[i+1:])
		}
	} else {
		n = atoi(num)
	}
	return n, t
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		if f, err2 := strconv.ParseFloat(s, 64); err2 == nil {
			return int(f)
		}
		return 0
	}
	return n
}

func parseYear(s string) int {
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		if n, err := strconv.Atoi(s[:4]); err == nil {
			return n
		}
	}
	return atoi(s)
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

func formatFromExt(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "flac", "mp3", "ogg", "opus", "wav", "aiff", "aif", "dsf", "dsd", "wma", "ape":
		if ext == "aif" {
			return "aiff"
		}
		return ext
	case "m4a", "m4b", "m4p", "mp4", "aac":
		return "m4a"
	}
	return ext
}
