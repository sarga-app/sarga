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
	return context.WithValue(
		context.WithValue(ctx, ctxKeyUserID, userID),
		ctxKeyIsAdmin, isAdmin,
	)
}

func UserIDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

func IsAdminFrom(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyIsAdmin).(bool)
	return v
}
