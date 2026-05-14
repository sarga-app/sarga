/** Catalog list page sizes — kept multiples of common column counts (4/5/6). */
export const ALBUMS_PAGE_SIZE = 60
export const ALBUMS_SKELETON_COUNT = 24
export const ARTISTS_PAGE_SIZE = 200
export const TRACKS_PAGE_SIZE = 100
export const PLAYLISTS_PAGE_SIZE = 50
export const SEARCH_PAGE_SIZE = 20

/** Loading skeleton delay — hide on fast loads to avoid flash. */
export const SKELETON_DELAY_MS = 200

/** Search input debounce. */
export const SEARCH_DEBOUNCE_MS = 300

/** Player bar layout (used by sidebar/main padding calc). */
export const PLAYER_HEIGHT = 76
export const PLAYER_GAP = 16

/** API base path. */
export const API_BASE = '/api/v1'

export type ArtSize = 160 | 320 | 480 | 640
export const artUrl = (hash: string, w?: ArtSize) =>
  w ? `/public/art/${hash}?w=${w}` : `/public/art/${hash}`
