package transport

import (
	"github.com/sarga-app/sarga/internal/transport/routes/api"
	"github.com/sarga-app/sarga/internal/transport/routes/health"
)

// Deps is the full set of cross-family dependencies the router composes.
// Per-family handlers are constructed in Run() before the router is built.
type Deps struct {
	DB  health.Pinger
	API api.Deps
}
