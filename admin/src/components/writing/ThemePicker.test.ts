import { describe, expect, it } from 'vitest'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import { readFileSync } from 'node:fs'

describe('ThemePicker', () => {
  const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'ThemePicker.vue'), 'utf8')

  it('renders the shared manifest and an explicit save action', () => {
    expect(source).toContain("import { THEMES, type ThemeName } from '@alive/theme'")
    expect(source).toContain('THEMES.map')
    expect(source).toContain('设为站点默认')
    expect(source).toContain('取消预览')
  })
})
