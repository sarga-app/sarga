package auth

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyIsAdmin
)

func WithUser(ctx context.Context, userID uuid.UUID, isAdmin bool) context.Context {
	withID := context.WithValue(ctx, ctxKeyUserID, userID)
	return context.WithValue(withID, ctxKeyIsAdmin, isAdmin)
}

func UserIDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

func IsAdminFrom(ctx context.Context) bool {
	if v, ok := ctx.Value(ctxKeyIsAdmin).(bool); ok {
		return v
	}
	return false
}
