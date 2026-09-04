import type { EntryDetail } from '../types/api'

export interface PublishCheck {
  field: 'title' | 'slug' | 'content_md' | 'summary' | 'category_id' | 'cover_url' | 'happened_at'
  label: string
  kind: 'blocker' | 'reminder'
}

const BLOCKERS: Array<[PublishCheck['field'], string]> = [
  ['title', '标题'],
  ['slug', 'slug'],
  ['content_md', '正文'],
]

const REMINDERS: Array<[PublishCheck['field'], string]> = [
  ['summary', '摘要'],
  ['category_id', '分类'],
  ['cover_url', '封面'],
]

export function getPublishChecks(entry: Pick<EntryDetail, 'title' | 'slug' | 'content_md' | 'summary' | 'category_id' | 'cover_url' | 'happened_at' | 'world'>): {
  blockers: PublishCheck[]
  reminders: PublishCheck[]
} {
  const missing = new Set<string>()
  if (entry.world !== 'saying' && entry.title.trim() === '') missing.add('title')
  if (entry.slug.trim() === '') missing.add('slug')
  if (entry.world !== 'video' && entry.content_md.trim() === '') missing.add('content_md')
  if (entry.world !== 'journal' && entry.summary.trim() === '') missing.add('summary')
  if (entry.world === 'journal' && entry.category_id === 0) missing.add('category_id')
  if (entry.world !== 'journal' && entry.cover_url.trim() === '') missing.add('cover_url')

  return {
    blockers: BLOCKERS.filter(([field]) => missing.has(field)).map(([field, label]) => ({ field, label, kind: 'blocker' })),
    reminders: REMINDERS.filter(([field]) => missing.has(field)).map(([field, label]) => ({ field, label, kind: 'reminder' })),
  }
}
