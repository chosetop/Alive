import { describe, expect, it } from 'vitest'

import { ADMIN_WORLD_REGISTRY, resolveAdminWorld } from './registry'

describe('resolveAdminWorld', () => {
  it('defines the supported admin worlds in fixed order', () => {
    expect(ADMIN_WORLD_REGISTRY.map((world) => world.key)).toEqual(['journal', 'saying', 'video'])
  })

  it('returns the Journal editor contract for the registered world', () => {
    expect(resolveAdminWorld('journal')).toMatchObject({
      editorKind: 'long-form',
      material: 'manuscript',
      description: '长文、图片与时间留下的痕迹',
      directoryNoun: '日志',
      emptyCopy: '还没有日志。写下第一篇。',
      searchPlaceholder: '搜索日志',
    })
  })

  it('returns the Saying editor contract for the registered world', () => {
    expect(resolveAdminWorld('saying')).toMatchObject({
      editorKind: 'saying',
      material: 'note',
      directoryNoun: '片语',
    })
  })

  it('returns the Video editor contract for the registered world', () => {
    expect(resolveAdminWorld('video')).toMatchObject({
      editorKind: 'long-form',
      material: 'viewfinder',
      directoryNoun: '影像',
    })
  })

  it('rejects unknown worlds', () => {
    expect(resolveAdminWorld('unknown')).toBeNull()
  })
})
