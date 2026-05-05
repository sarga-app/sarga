package server

import (
	"github.com/sarga-app/sarga/internal/server/api"
	"github.com/sarga-app/sarga/internal/server/health"
)

// Deps is the full set of cross-family dependencies the router composes.
// Per-family handlers are constructed in Run() before the router is built.
type Deps struct {
	DB  health.Pinger
	API api.Deps
}
