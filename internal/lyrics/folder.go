package lyrics

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxAdjacentBytes = 256 * 1024

var lrcTimestamp = regexp.MustCompile(`\[\d{1,2}:\d{2}(?:[.:]\d{1,3})?\]`)

func ReadAdjacent(audioPath string) (*Lyrics, error) {
	lrcPath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".lrc"
	info, err := os.Stat(lrcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // expected: most tracks don't have .lrc
		}
		return nil, err
	}
	if info.Size() > maxAdjacentBytes {
		return nil, nil //nolint:nilnil // skip absurdly large files
	}
	b, err := os.ReadFile(lrcPath) //nolint:gosec
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(string(b))
	if content == "" {
		return nil, nil //nolint:nilnil
	}
	return &Lyrics{
		Content: content,
		Source:  SourceSidecar,
		Synced:  lrcTimestamp.MatchString(content),
	}, nil
}
