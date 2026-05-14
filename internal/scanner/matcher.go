package scanner

import (
	"context"

	"github.com/sarga-app/sarga/internal/catalog"
	"github.com/sarga-app/sarga/internal/providers"
)

const matchThreshold = 0.85

type Matcher struct {
	mb       providers.IdentificationProvider
	discogs  providers.IdentificationProvider
	acoustid providers.IdentificationProvider
}

func NewMatcher(mb, discogs, acoustid providers.IdentificationProvider) *Matcher {
	return &Matcher{mb: mb, discogs: discogs, acoustid: acoustid}
}

func (m *Matcher) Match(ctx context.Context, meta catalog.Metadata) *providers.MatchResult {
	if meta.HasMBID() {
		return &providers.MatchResult{
			Confidence:    1.0,
			Source:        "mbid_tag",
			MBRecordingID: meta.MBRecordingID,
			MBReleaseID:   meta.MBReleaseID,
			MBArtistID:    meta.MBArtistID,
		}
	}

	for _, p := range []providers.IdentificationProvider{m.mb, m.discogs, m.acoustid} {
		if p == nil {
			continue
		}
		r, err := p.Match(ctx, meta)
		if err == nil && r != nil && r.Confidence >= matchThreshold {
			return r
		}
	}
	return nil
}
