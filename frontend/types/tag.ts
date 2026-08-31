export type Tag = { id: number; name: string; slug: string; usage_count?: number }

export type TaggedEntry = {
  world: string
  kind: string
  slug: string
  title?: string
  excerpt?: string
  cover_url?: string
  saying?: { content_md: string; short_id: string }
}

export type TagEntriesPage = { tag: Tag; items: TaggedEntry[]; meta: { page: number; page_size: number; total: number } }
