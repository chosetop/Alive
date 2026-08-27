import { describe, expect, it } from 'vitest'

import { resolveVisitorTheme } from './useTheme'

describe('resolveVisitorTheme', () => {
  it('uses the site default when there is no visitor cookie', () => {
    expect(resolveVisitorTheme(null, 'codex-lavender')).toBe('codex-lavender')
  })

  it('lets a valid visitor choice override the site default', () => {
    expect(resolveVisitorTheme('lamp', 'ink')).toBe('lamp')
  })

  it('ignores an invalid visitor cookie', () => {
    expect(resolveVisitorTheme('bogus', 'lamp')).toBe('lamp')
  })

  it('falls back to ink when both values are invalid or unavailable', () => {
    expect(resolveVisitorTheme(null, 'bogus')).toBe('ink')
  })
})
