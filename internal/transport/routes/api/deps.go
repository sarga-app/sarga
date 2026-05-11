package api

import (
	"github.com/sarga-app/sarga/internal/transport/routes/api/auth"
	"github.com/sarga-app/sarga/internal/transport/routes/api/catalog"
	"github.com/sarga-app/sarga/internal/transport/routes/api/docs"
	"github.com/sarga-app/sarga/internal/transport/routes/api/libraries"
	"github.com/sarga-app/sarga/internal/transport/routes/art"
)

// Deps bundles the per-domain handlers /api/v1 dispatches to.
type Deps struct {
	Auth      *auth.Handler
	Docs      *docs.Handler
	Libraries *libraries.Handler
	Catalog   *catalog.Handler
	Art       *art.Handler
}
