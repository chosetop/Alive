export type SayingView = 'stream' | 'wall' | 'focus'

export interface SayingListItem {
  short_id: string
  content_md: string
  source?: string
  author?: string
  category?: { id: number; name: string; slug: string } | null
}

export interface SayingDetail extends SayingListItem {
  previous?: { short_id: string }
  next?: { short_id: string }
}
