import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useWritingStore } from './writing'
import type { EntryListItem } from '../types/api'

function item(overrides: Partial<EntryListItem> = {}): EntryListItem {
  return {
    id: 1,
    world: 'journal',
    kind: '',
    type: 'journal',
    title: '山中一日',
    slug: 'a-day',
    summary: '',
    cover_url: '',
    meta: {},
    word_count: 100,
    category: null,
    happened_at: null,
    published_at: null,
    status: 'draft',
    visibility: 'public',
    created_at: '2026-08-20T00:00:00Z',
    updated_at: '2026-08-25T10:00:00Z',
    ...overrides,
  }
}

describe('the writing store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('opens the directory by default', () => {
    // A workspace that started collapsed would hide the only route to the other
    // articles from someone who has never seen the collapse control.
    expect(useWritingStore().directoryOpen).toBe(true)
  })

  it('toggles the directory in both directions', () => {
    const store = useWritingStore()

    store.toggleDirectory()
    expect(store.directoryOpen).toBe(false)

    store.toggleDirectory()
    expect(store.directoryOpen).toBe(true)
  })

  it('sets directory visibility to an absolute value', () => {
    const store = useWritingStore()

    // The mobile drawer closes after a selection whether or not it was open, so
    // it needs a setter rather than a toggle.
    store.setDirectoryOpen(false)
    store.setDirectoryOpen(false)
    expect(store.directoryOpen).toBe(false)
  })

  it('starts with no active article and accepts one being cleared again', () => {
    const store = useWritingStore()

    expect(store.activeEntryId).toBeNull()

    store.setActiveEntry(41)
    expect(store.activeEntryId).toBe(41)

    // Cleared when the editor unmounts, so a stale row is not left highlighted
    // over a canvas showing something else.
    store.setActiveEntry(null)
    expect(store.activeEntryId).toBeNull()
  })

  it('holds the directory search text', () => {
    const store = useWritingStore()

    expect(store.searchQuery).toBe('')

    store.setSearchQuery('山中')
    expect(store.searchQuery).toBe('山中')
  })

  it('switches the active world and resets world-scoped directory state', () => {
    const store = useWritingStore()

    store.setActiveWorld('journal')
    store.setActiveEntry(41)
    store.setSearchQuery('雨')
    store.setDirectoryEntries([item()])

    store.setActiveWorld('saying')

    expect(store.activeWorld).toBe('saying')
    expect(store.activeEntryId).toBeNull()
    expect(store.searchQuery).toBe('')
    expect(store.directoryEntries).toEqual([])
  })

  it('owns no document content or autosave state', () => {
    // The plan's module boundary, asserted at source level because it is a
    // statement about what the store must never grow, and no runtime assertion
    // can observe the absence of a field that was never added.
    //
    // Two owners for one document is not a failure anything in the UI reveals:
    // the store and the coordinator would each believe they held the current
    // text, and whichever wrote second would silently win.
    // Comments are stripped first. The file explains at length why it does not
    // own the document, and a naive substring scan would fail on the explanation
    // rather than on the code -- which would make the guard unmaintainable and
    // teach the next person to delete it.
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'writing.ts'),
      'utf8',
    )
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/\/\/.*$/gm, '')

    for (const forbidden of ['contentMd', 'content_md', 'revision', 'oordinator', 'arkdown']) {
      expect(source, `writing store must not mention ${forbidden}`).not.toContain(forbidden)
    }
  })
})
