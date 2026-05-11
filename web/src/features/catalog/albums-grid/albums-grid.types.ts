export type AlbumSort = 'date' | 'title' | 'added'

export interface AlbumsGridProps {
  libraryId: string
  page: number
  sort: AlbumSort
  compilations: boolean
  onPageChange: (p: number) => void
  onSortChange: (s: AlbumSort) => void
  onCompilationsChange: (c: boolean) => void
}
