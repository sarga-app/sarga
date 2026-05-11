export interface PaginationProps {
  page: number
  totalPages: number
  onChange: (p: number) => void
}
