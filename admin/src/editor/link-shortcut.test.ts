import { describe, expect, it } from 'vitest'

import { isLinkShortcut } from './link-shortcut'

describe('link shortcut', () => {
  it('recognizes Cmd/Ctrl-K and ignores other keys', () => {
    expect(isLinkShortcut({ key: 'k', metaKey: true, ctrlKey: false })).toBe(true)
    expect(isLinkShortcut({ key: 'K', metaKey: false, ctrlKey: true })).toBe(true)
    expect(isLinkShortcut({ key: 'k', metaKey: false, ctrlKey: false })).toBe(false)
  })
})
