package api

import (
	"github.com/sarga-app/sarga/internal/transport/routes/api/auth"
	"github.com/sarga-app/sarga/internal/transport/routes/api/gen"
	"github.com/sarga-app/sarga/internal/transport/routes/api/libraries"
)

// Compile-time assertion: every operation in api/openapi.yaml is handled.
// Add new alias + embedded field here as sub-domains are implemented.
var _ gen.ServerInterface = (*apiHandler)(nil)

type (
	authHandler = auth.Handler
	libsHandler = libraries.Handler
)

type apiHandler struct {
	*authHandler
	*libsHandler
}
