import { lazy } from 'react'
import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'

const LibraryPage = lazy(() =>
  import('@/features/library/library-page').then((m) => ({ default: m.LibraryPage })),
)

export const libraryRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/library',
  component: LibraryPage,
})
