import { describe, expect, it } from 'vitest'

import { isSlashMenuContext, slashMenuPlugin, slashQueryRange } from './slash-plugin'

describe('Milkdown slash plugin', () => {
  it('only enables the menu for a paragraph slash context', () => {
    expect(isSlashMenuContext({ textBeforeCaret: '/标题', nodeType: 'paragraph', atEnd: true })).toBe(true)
    expect(isSlashMenuContext({ textBeforeCaret: '/标题', nodeType: 'code_block', atEnd: true })).toBe(false)
    expect(isSlashMenuContext({ textBeforeCaret: '/标题', nodeType: 'paragraph', atEnd: false })).toBe(false)
  })

  it('exports a Milkdown plugin', () => {
    expect(slashMenuPlugin).toHaveLength(2)
  })

  it('calculates the slash text range before focus moves to the menu', () => {
    expect(slashQueryRange(12, '标题')).toEqual({ from: 9, to: 12 })
  })
})
