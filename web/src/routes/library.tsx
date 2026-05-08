import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'
import { LibraryPage } from '@/features/library/library-page'

export const libraryRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/library',
  component: LibraryPage,
})
