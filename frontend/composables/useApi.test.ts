import { describe, expect, it } from 'vitest'
import { resolveApiBase } from '../utils/api-base'

describe('resolveApiBase', () => {
  it('uses the local reverse proxy during SSR when no public origin is configured', () => {
    expect(resolveApiBase('', true, 'https://p30s-mac-mini.tail3a5bd9.ts.net')).toBe(
      'http://127.0.0.1:8081',
    )
  })

  it('uses the browser origin when no public origin is configured', () => {
    expect(resolveApiBase('', false, 'https://p30s-mac-mini.tail3a5bd9.ts.net')).toBe(
      'https://p30s-mac-mini.tail3a5bd9.ts.net',
    )
  })

  it('points the direct Nuxt development port at the Go API', () => {
    expect(resolveApiBase('', false, 'http://localhost:3000')).toBe('http://127.0.0.1:8080')
  })

  it('preserves an explicitly configured origin', () => {
    expect(resolveApiBase('http://localhost:8080', true, 'https://example.test')).toBe(
      'http://localhost:8080',
    )
  })
})
