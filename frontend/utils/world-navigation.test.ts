import { describe, expect, it } from 'vitest'

import type { PublicWorldSetting } from '~/types'
import { buildWorldNavigation } from './world-navigation'

describe('buildWorldNavigation', () => {
  it('keeps open worlds in configured order and resolves their public routes', () => {
    expect(buildWorldNavigation([
      { world: 'video', nav_label: '影像', sort_order: 30, default_view: '' },
      { world: 'journal', nav_label: '日志', sort_order: 10, default_view: '' },
      { world: 'unknown', nav_label: '未知', sort_order: 20, default_view: '' } as unknown as PublicWorldSetting,
    ] as PublicWorldSetting[])).toEqual([
      { key: 'journal', label: '日志', rootPath: '/journal' },
      { key: 'video', label: '影像', rootPath: '/videos' },
    ])
  })
})
