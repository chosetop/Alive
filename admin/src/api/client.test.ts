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

  it('maps a local API origin to the current host for LAN development', () => {
    expect(resolveApiOrigin('http://127.0.0.1:8080', 'http://192.168.1.48:5173')).toBe(
      'http://192.168.1.48:8080',
    )
  })

  it('maps a local API origin to the Tailscale host for remote development', () => {
    expect(
      resolveApiOrigin('http://127.0.0.1:8080', 'http://p30s-mac-mini.tail3a5bd9.ts.net:5173'),
    ).toBe('http://p30s-mac-mini.tail3a5bd9.ts.net:8080')
  })
})
