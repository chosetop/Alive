import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('MarkdownEditor Milkdown controls', () => {
  it('does not mount the bottom link paste tooltip', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).not.toContain('configureLinkTooltip')
    expect(source).not.toContain('linkTooltipPlugin')
    expect(source).not.toContain('Paste link...')
  })

  it('handles Escape at document level for editor overlays', () => {
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

  it('keeps Markdown on a quiet, measured prose surface', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).toContain('data-editor-visual-mode="quiet-paper"')
    expect(source).toMatch(/\.milkdown\)[\s\S]*max-width:\s*42rem/)
    expect(source).toMatch(/\.ProseMirror\)[\s\S]*font-family:\s*var\(--font-prose\)/)
    expect(source).toContain('caret-color: var(--c-alive)')
  })

})
