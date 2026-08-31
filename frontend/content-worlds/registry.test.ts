import { describe, expect, it } from 'vitest'

import { listPublicWorlds, resolvePublicWorld } from './registry'

describe('public world registry', () => {
  it('resolves journal to the public journal route set', () => {
    const journal = resolvePublicWorld('journal')

    expect(journal).toMatchObject({
      key: 'journal',
      label: '日志',
      rootPath: '/journal',
      timePolicy: 'timeline',
      seoType: 'Article',
    })
    expect(journal?.entryPath('spring-river')).toBe('/journal/spring-river')
    expect(journal?.categoryPath('travel')).toBe('/journal/categories/travel')
  })

  it('only lists registered public worlds', () => {
    expect(listPublicWorlds().map((world) => world.key)).toEqual(['journal', 'saying'])
    expect(resolvePublicWorld('saying')).toMatchObject({
      key: 'saying',
      label: '片语',
      rootPath: '/sayings',
      timePolicy: 'hidden',
      seoType: 'SocialMediaPosting',
    })
    expect(resolvePublicWorld('video')).toBeNull()
    expect(resolvePublicWorld('unknown')).toBeNull()
  })
})
