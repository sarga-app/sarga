package lyrics

type Source string

const (
	SourceEmbedded Source = "embedded"
	SourceSidecar  Source = "sidecar"
	SourceLRCLib   Source = "lrclib"
	SourceManual   Source = "manual"
)

type Lyrics struct {
	Content string
	Source  Source
	Synced  bool
}
