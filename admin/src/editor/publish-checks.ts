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
  ['happened_at', '发生时间'],
]

export function getPublishChecks(entry: Pick<EntryDetail, 'title' | 'slug' | 'content_md' | 'summary' | 'category_id' | 'cover_url' | 'happened_at'>): {
  blockers: PublishCheck[]
  reminders: PublishCheck[]
} {
  const missing = new Set<string>()
  if (entry.title.trim() === '') missing.add('title')
  if (entry.slug.trim() === '') missing.add('slug')
  if (entry.content_md.trim() === '') missing.add('content_md')
  if (entry.summary.trim() === '') missing.add('summary')
  if (entry.category_id === 0) missing.add('category_id')
  if (entry.cover_url.trim() === '') missing.add('cover_url')
  if (entry.happened_at === null) missing.add('happened_at')

  return {
    blockers: BLOCKERS.filter(([field]) => missing.has(field)).map(([field, label]) => ({ field, label, kind: 'blocker' })),
    reminders: REMINDERS.filter(([field]) => missing.has(field)).map(([field, label]) => ({ field, label, kind: 'reminder' })),
  }
}
