import { describe, expect, it } from 'vitest'

import { getEntryNeighbors } from './entry-navigation'

const entries = [
  { slug: 'newest', title: '最新', published_at: '2026-08-27T10:00:00Z' },
  { slug: 'current', title: '当前', published_at: '2026-08-26T10:00:00Z' },
  { slug: 'oldest', title: '最早', published_at: '2026-08-25T10:00:00Z' },
]

describe('getEntryNeighbors', () => {
  it('returns newer as previous and older as next', () => {
    expect(getEntryNeighbors(entries, 'current')).toEqual({
      previous: entries[0],
      next: entries[2],
    })
  })

  it('leaves the missing edge empty', () => {
    expect(getEntryNeighbors(entries, 'newest')).toEqual({ previous: null, next: entries[1] })
    expect(getEntryNeighbors(entries, 'oldest')).toEqual({ previous: entries[1], next: null })
  })
})
