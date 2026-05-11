package library

import (
	"context"
	"math"
	"strings"
)

const matchThreshold = 0.85

type Matcher struct {
	mb       IdentificationProvider
	discogs  IdentificationProvider
	acoustid IdentificationProvider
}

func NewMatcher(mb, discogs, acoustid IdentificationProvider) *Matcher {
	return &Matcher{mb: mb, discogs: discogs, acoustid: acoustid}
}

func (m *Matcher) Match(ctx context.Context, meta Metadata) (*MatchResult, error) {
	if meta.HasMBID() {
		return &MatchResult{
			Confidence:    1.0,
			Source:        "mbid_tag",
			MBRecordingID: meta.MBRecordingID,
			MBReleaseID:   meta.MBReleaseID,
			MBArtistID:    meta.MBArtistID,
		}, nil
	}

	if m.mb != nil {
		r, err := m.mb.Match(ctx, meta)
		if err == nil && r != nil && r.Confidence >= matchThreshold {
			return r, nil
		}
	}

	if m.discogs != nil {
		r, err := m.discogs.Match(ctx, meta)
		if err == nil && r != nil && r.Confidence >= matchThreshold {
			return r, nil
		}
	}

	if m.acoustid != nil {
		r, err := m.acoustid.Match(ctx, meta)
		if err == nil && r != nil && r.Confidence >= matchThreshold {
			return r, nil
		}
	}

	return nil, nil //nolint:nilnil // nil = "no match found, not an error"
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

func mbFuzzyScore(meta Metadata, title, artist, album string, year int, durationMs int64) float64 {
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
