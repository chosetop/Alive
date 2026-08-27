import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'

import { THEMES, isThemeName, resolveTheme } from './index'

describe('shared theme manifest', () => {
  it('keeps codex lavender labelled 薰衣草 and publishes night ink', () => {
    expect(THEMES).toContainEqual({ name: 'codex-lavender', label: '薰衣草', colorScheme: 'light' })
    expect(THEMES).toContainEqual({ name: 'night-ink', label: '夜航', colorScheme: 'dark' })
  })

  it('validates theme names and resolves visitor preference first', () => {
    expect(isThemeName('lamp')).toBe(true)
    expect(isThemeName('night-ink')).toBe(true)
    expect(isThemeName('bogus')).toBe(false)
    expect(resolveTheme({ visitor: 'lamp', siteDefault: 'ink' })).toBe('lamp')
    expect(resolveTheme({ visitor: 'night-ink', siteDefault: 'ink' })).toBe('night-ink')
    expect(resolveTheme({ visitor: null, siteDefault: 'codex-lavender' })).toBe('codex-lavender')
  })

  it('treats invalid values as absent and falls back to ink', () => {
    expect(resolveTheme({ visitor: 'bogus', siteDefault: 'ink' })).toBe('ink')
    expect(resolveTheme({ visitor: 'bogus', siteDefault: 'also-bogus' })).toBe('ink')
  })

  it('defines every semantic token for every published theme', () => {
    const css = readFileSync(new URL('./themes.css', import.meta.url), 'utf8')
    const tokens = [
      '--font-prose', '--font-heading', '--font-ui', '--font-mono',
      '--c-paper', '--c-surface', '--c-surface-sunken', '--c-glass', '--c-glass-border',
      '--c-ink', '--c-prose', '--c-ink-muted', '--c-ink-faint', '--c-line', '--c-line-strong',
      '--c-accent', '--c-accent-hover', '--c-focus', '--c-success', '--c-success-surface',
      '--c-danger', '--c-danger-surface', '--c-on-accent', '--c-overlay',
    ]

    for (const theme of THEMES) {
      const selector = theme.name === 'ink'
        ? /:root,\s*\[data-theme='ink'\] \{([\s\S]*?)\n\}/
        : new RegExp(`\\[data-theme='${theme.name}'\\] \\{([\\s\\S]*?)\\n\\}`)
      const palette = css.match(selector)?.[1]

      expect(palette, `${theme.name} palette`).toBeDefined()
      for (const token of tokens) {
        expect(palette, `${theme.name} ${token}`).toContain(`${token}:`)
      }
    }
  })
})
