import { describe, expect, it } from 'vitest'

import { THEMES, isThemeName, resolveTheme } from './index'

describe('shared theme manifest', () => {
  it('keeps the initial theme names and labels stable', () => {
    expect(THEMES).toEqual([
      { name: 'ink', label: '纸墨', colorScheme: 'light' },
      { name: 'lamp', label: '灯下', colorScheme: 'dark' },
      { name: 'codex-lavender', label: '薰衣草', colorScheme: 'light' },
    ])
  })

  it('validates theme names and resolves visitor preference first', () => {
    expect(isThemeName('lamp')).toBe(true)
    expect(isThemeName('bogus')).toBe(false)
    expect(resolveTheme({ visitor: 'lamp', siteDefault: 'ink' })).toBe('lamp')
    expect(resolveTheme({ visitor: null, siteDefault: 'codex-lavender' })).toBe('codex-lavender')
  })

  it('treats invalid values as absent and falls back to ink', () => {
    expect(resolveTheme({ visitor: 'bogus', siteDefault: 'ink' })).toBe('ink')
    expect(resolveTheme({ visitor: 'bogus', siteDefault: 'also-bogus' })).toBe('ink')
  })
})
