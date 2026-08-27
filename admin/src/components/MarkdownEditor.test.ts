import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('MarkdownEditor Milkdown controls', () => {
  it('configures the link tooltip before mounting its plugin', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'MarkdownEditor.vue'), 'utf8')

    expect(source).toContain('configureLinkTooltip(ctx)')
  })
})
