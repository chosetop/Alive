import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('App', () => {
  it('provides Milkdown context to editor routes', () => {
    const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'App.vue'), 'utf8')

    expect(source).toContain("import { MilkdownProvider } from '@milkdown/vue'")
    expect(source).toMatch(/<MilkdownProvider>[\s\S]*<RouterView \/>[\s\S]*<\/MilkdownProvider>/)
  })
})
