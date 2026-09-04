import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'

const source = readFileSync('src/views/SayingEditor.vue', 'utf8')

describe('SayingEditor presentation', () => {
  it('does not render permalink guidance or controls', () => {
    expect(source).not.toContain('permalink-hint')
    expect(source).not.toContain('data-permalink')
    expect(source).not.toContain('copyPermalink')
  })
})
