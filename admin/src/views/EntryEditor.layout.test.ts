import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('EntryEditor writing surface', () => {
  it('gives the page an explicit responsive width instead of intrinsic sizing', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'EntryEditor.vue'), 'utf8')

    expect(source).toMatch(/\.page\s*\{[\s\S]*width:\s*min\(100%, 52rem\)/)
  })

  it('keeps the title and delete action out of the main writing canvas', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'EntryEditor.vue'), 'utf8')

    expect(source).not.toContain('class="input input--title"')
    expect(source).not.toContain('<div class="actions">')
    expect(source).toContain('@delete="void handleDelete()"')
  })

  it('does not render redundant body labels or markdown guidance', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'EntryEditor.vue'), 'utf8')

    expect(source).not.toContain('<label class="label">正文</label>')
    expect(source).not.toContain('所见即所得，存的是 Markdown 源文本。')
  })
})
