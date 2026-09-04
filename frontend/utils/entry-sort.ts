import type { EntryListItem } from '~/types'

export type EntryCreatedSort = 'newest' | 'oldest'

export function sortEntriesByCreatedAt(entries: EntryListItem[], order: EntryCreatedSort): EntryListItem[] {
  return entries.toSorted((left, right) => {
    const delta = Date.parse(left.created_at) - Date.parse(right.created_at)
    return order === 'oldest' ? delta : -delta
  })
}
