import type { LucideIcon } from 'lucide-react'

export interface EmptyStateProps {
  icon: LucideIcon
  title: string
  body?: string
}

export interface ErrorStateProps {
  message: string
  onRetry?: () => void
}

export interface ComingSoonProps {
  icon: LucideIcon
  title: string
  body?: string
}
