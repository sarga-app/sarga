package api

import "github.com/sarga-app/sarga/internal/server/api/auth"

// Deps bundles the per-domain handlers /api/v1 dispatches to. Add a field per
// new sub-domain (libraries, playlists, ...) as they're implemented.
type Deps struct {
	Auth *auth.Handler
}
