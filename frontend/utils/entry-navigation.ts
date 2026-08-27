import type { EntryListItem } from '~/types'

export type EntryNavigationItem = Pick<EntryListItem, 'slug' | 'title' | 'published_at'>

export type EntryNeighbors = {
  /** The newer entry in the public list order. */
  previous: EntryNavigationItem | null
  /** The older entry in the public list order. */
  next: EntryNavigationItem | null
}

/**
 * The public API orders entries newest first. Keep that order's neighboring
 * items, without sorting locally and risking a mismatch with the index page.
 */
export function getEntryNeighbors(
  entries: EntryNavigationItem[],
  currentSlug: string,
): EntryNeighbors {
  const index = entries.findIndex((entry) => entry.slug === currentSlug)
  if (index < 0) return { previous: null, next: null }

  return {
    previous: entries[index - 1] ?? null,
    next: entries[index + 1] ?? null,
  }
}
