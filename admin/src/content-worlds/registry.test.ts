import { describe, expect, it } from 'vitest'

import { ADMIN_WORLD_REGISTRY, resolveAdminWorld } from './registry'

describe('resolveAdminWorld', () => {
  it('defines the supported admin worlds in fixed order', () => {
    expect(ADMIN_WORLD_REGISTRY.map((world) => world.key)).toEqual(['journal', 'saying', 'video'])
  })

  it('returns the Journal editor contract for the registered world', () => {
    expect(resolveAdminWorld('journal')).toMatchObject({
      key: 'journal',
      label: '日志',
      createLabel: '写日志',
      editorRouteName: 'entry-new-world',
      publishPolicy: 'journal',
      categoryEnabled: true,
      mediaCapability: 'markdown-images',
    })
  })

  it('rejects unknown worlds', () => {
    expect(resolveAdminWorld('unknown')).toBeNull()
  })
})
