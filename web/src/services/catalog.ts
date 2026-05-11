import { api } from './api'

export async function listLibraries() {
  const { data, error } = await api.GET('/libraries')
  if (error) throw error
  return data
}

export async function triggerScan(libraryId: string) {
  const { data, error } = await api.POST('/libraries/{libraryId}/scan', {
    params: { path: { libraryId } },
  })
  if (error) throw error
  return data
}

export async function listScanRuns(libraryId: string, page = 1, limit = 20) {
  const { data, error } = await api.GET('/libraries/{libraryId}/scan-runs', {
    params: { path: { libraryId }, query: { page, limit } },
  })
  if (error) throw error
  return data
}

export async function getLibrarySettings(libraryId: string) {
  const { data, error } = await api.GET('/libraries/{libraryId}/settings', {
    params: { path: { libraryId } },
  })
  if (error) throw error
  return data
}

export async function putLibrarySettings(libraryId: string, body: Record<string, unknown>) {
  const { data, error } = await api.PUT('/libraries/{libraryId}/settings', {
    params: { path: { libraryId } },
    body: body as never,
  })
  if (error) throw error
  return data
}

export async function listAlbums(libraryId: string, opts?: {
  sort?: 'date' | 'title' | 'added'
  compilations_only?: boolean
  page?: number
  limit?: number
}) {
  const { data, error } = await api.GET('/albums', {
    params: { query: { library_id: libraryId, ...opts } },
  })
  if (error) throw error
  return data
}

export async function getAlbum(id: string, libraryId: string, page = 1, limit = 100) {
  const { data, error } = await api.GET('/albums/{id}', {
    params: { path: { id }, query: { library_id: libraryId, page, limit } },
  })
  if (error) throw error
  return data
}

export async function listArtists(libraryId: string, page = 1, limit = 50) {
  const { data, error } = await api.GET('/artists', {
    params: { query: { library_id: libraryId, page, limit } },
  })
  if (error) throw error
  return data
}

export async function getArtist(id: string, libraryId: string, page = 1, limit = 50) {
  const { data, error } = await api.GET('/artists/{id}', {
    params: { path: { id }, query: { library_id: libraryId, page, limit } },
  })
  if (error) throw error
  return data
}

export async function listPlaylists(libraryId: string, page = 1, limit = 50) {
  const { data, error } = await api.GET('/playlists', {
    params: { query: { library_id: libraryId, page, limit } },
  })
  if (error) throw error
  return data
}

export async function getPlaylist(id: string, libraryId: string, page = 1, limit = 100) {
  const { data, error } = await api.GET('/playlists/{id}', {
    params: { path: { id }, query: { library_id: libraryId, page, limit } },
  })
  if (error) throw error
  return data
}

export async function listTracks(libraryId: string, page = 1, limit = 100) {
  const { data, error } = await api.GET('/tracks', {
    params: { query: { library_id: libraryId, page, limit } },
  })
  if (error) throw error
  return data
}

export async function search(q: string, libraryId: string, page = 1, limit = 20, signal?: AbortSignal) {
  const { data, error } = await api.GET('/search', {
    params: { query: { q, library_id: libraryId, page, limit } },
    signal,
  })
  if (error) throw error
  return data
}
