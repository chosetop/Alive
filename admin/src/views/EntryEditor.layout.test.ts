import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('EntryEditor writing surface', () => {
  it('gives the page an explicit responsive width instead of intrinsic sizing', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'EntryEditor.vue'), 'utf8')

    expect(source).toMatch(/\.page\s*\{[\s\S]*width:\s*min\(100%, 52rem\)/)
  })
})
