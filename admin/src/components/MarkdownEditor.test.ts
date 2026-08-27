import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('MarkdownEditor Milkdown controls', () => {
  it('configures the link tooltip before mounting its plugin', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).toContain('configureLinkTooltip(ctx)')
  })

  it('handles Escape at document level so portal tooltips can close', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).toContain("event.key === 'Escape'")
    expect(source).toContain("document.body.dispatchEvent(new Event('pointerdown'")
  })

  it('does not mount a slash command menu', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).not.toContain('slashMenuPlugin')
    expect(source).not.toContain('configureSlashMenu')
  })

  it('gives the editor a stable full-width writing surface', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).toContain('width: 100%')
    expect(source).toContain('min-width: 0')
  })

  it('supports collapsing heading sections without changing Markdown content', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).toContain('toggleHeadingSection')
    expect(source).toContain('data-heading-collapsed')
  })
})
