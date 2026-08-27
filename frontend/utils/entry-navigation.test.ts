import { describe, expect, it } from 'vitest'

import { getEntryNavigationPages, getEntryNeighborTitle, getEntryNeighbors } from './entry-navigation'

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

  it('uses friendly prompts for the missing edge directions', () => {
    const newest = getEntryNeighbors(entries, 'newest')
    const oldest = getEntryNeighbors(entries, 'oldest')

    expect(getEntryNeighborTitle('previous', newest.previous)).toBe('已经是第一篇')
    expect(getEntryNeighborTitle('next', oldest.next)).toBe('已经是最后一篇')
    expect(getEntryNeighborTitle('previous', oldest.previous)).toBe('当前')
  })

  it('does not present an API failure as an article boundary', () => {
    expect(getEntryNeighborTitle('previous', null, false)).toBe('暂时无法获取')
    expect(getEntryNeighborTitle('next', null, false)).toBe('暂时无法获取')
  })

  it('requests every public list page needed for navigation', () => {
    expect(getEntryNavigationPages(121, 50)).toEqual([1, 2, 3])
    expect(getEntryNavigationPages(50, 50)).toEqual([1])
  })
})
