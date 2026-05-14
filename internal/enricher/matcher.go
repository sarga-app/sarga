package enricher

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

func (m *Matcher) Match(ctx context.Context, meta catalog.Metadata) (*providers.MatchResult, error) {
	if meta.HasMBID() {
		return &providers.MatchResult{
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
