import { describe, expect, it } from 'vitest'

import { resolveApiOrigin } from './client'

describe('resolveApiOrigin', () => {
  it('uses the current origin when no cross-origin API is configured', () => {
    expect(resolveApiOrigin(undefined, 'https://alive.example')).toBe('https://alive.example')
    expect(resolveApiOrigin('', 'http://localhost:8081')).toBe('http://localhost:8081')
  })

  it('keeps an explicitly configured API origin for development', () => {
    expect(resolveApiOrigin('http://localhost:8080/', 'https://alive.example')).toBe(
      'http://localhost:8080',
    )
  })
})
