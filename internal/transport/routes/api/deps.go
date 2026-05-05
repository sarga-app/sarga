package api

import (
	"github.com/sarga-app/sarga/internal/transport/routes/api/auth"
	"github.com/sarga-app/sarga/internal/transport/routes/api/docs"
)

// Deps bundles the per-domain handlers /api/v1 dispatches to. Add a field per
// new sub-domain (libraries, playlists, ...) as they're implemented.
type Deps struct {
	Auth *auth.Handler
	Docs *docs.Handler
}
