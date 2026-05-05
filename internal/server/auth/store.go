package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/platform/db/queries"
)

// Store is the narrow DB surface the auth handlers need; tests can fake it.
type Store interface {
	GetUserByUsername(ctx context.Context, username string) (queries.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (queries.User, error)
	UpdateUserPassword(ctx context.Context, arg queries.UpdateUserPasswordParams) error

	CreateRefreshToken(ctx context.Context, arg queries.CreateRefreshTokenParams) (queries.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (queries.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error
}
