import { createRouter } from '@tanstack/react-router'
import { rootRoute } from '@/routes/root'
import { indexRoute } from '@/routes/index'
import { loginRoute } from '@/routes/login'
import { authRoute } from '@/routes/auth'
import { libraryRoute } from '@/routes/library'
import { albumsRoute } from '@/routes/albums'
import { albumDetailRoute } from '@/routes/albums.$id'
import { artistsRoute } from '@/routes/artists'
import { artistDetailRoute } from '@/routes/artists.$id'
import { playlistsRoute } from '@/routes/playlists'
import { playlistDetailRoute } from '@/routes/playlists.$id'
import { searchRoute } from '@/routes/search'
import { tracksRoute } from '@/routes/tracks'
import { homeRoute } from '@/routes/home'
import { browseRoute } from '@/routes/browse'
import { radioRoute } from '@/routes/radio'
import { lovedRoute } from '@/routes/loved'
import { queryClient } from './query-client'

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  authRoute.addChildren([
    homeRoute,
    browseRoute,
    radioRoute,
    lovedRoute,
    libraryRoute,
    albumsRoute,
    albumDetailRoute,
    artistsRoute,
    artistDetailRoute,
    playlistsRoute,
    playlistDetailRoute,
    tracksRoute,
    searchRoute,
  ]),
])

export const router = createRouter({
  routeTree,
  scrollRestoration: true,
  context: {
    queryClient,
    auth: { user: null, accessToken: null },
  },
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
