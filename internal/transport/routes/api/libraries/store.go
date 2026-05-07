package libraries

import (
	"context"

	"github.com/sarga-app/sarga/internal/support/db/queries"
)

type Store interface {
	CreateLibrary(ctx context.Context, arg queries.CreateLibraryParams) (queries.Library, error)
	GetLibraryByID(ctx context.Context, id string) (queries.Library, error)
	ListLibraries(ctx context.Context) ([]queries.Library, error)
	ListLibrariesForUser(ctx context.Context, userID string) ([]queries.Library, error)
	UpdateLibrary(ctx context.Context, arg queries.UpdateLibraryParams) (queries.Library, error)
	DeleteLibrary(ctx context.Context, id string) error
	GrantLibraryAccess(ctx context.Context, arg queries.GrantLibraryAccessParams) error
	RevokeLibraryAccess(ctx context.Context, arg queries.RevokeLibraryAccessParams) error
	GetLibraryAccess(ctx context.Context, arg queries.GetLibraryAccessParams) (queries.LibraryAccess, error)
	ListLibraryMembers(ctx context.Context, libraryID string) ([]queries.ListLibraryMembersRow, error)
}
