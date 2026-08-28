import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { getDashboardMetrics } from './dashboard'

describe('getDashboardMetrics', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { total_entries: 12, published_entries: 8, total_words: 3456 },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loads dashboard metrics from the admin dashboard endpoint', async () => {
    await expect(getDashboardMetrics()).resolves.toEqual({
      total_entries: 12,
      published_entries: 8,
      total_words: 3456,
    })

    expect(String(fetchMock.mock.calls[0]?.[0])).toContain('/api/v1/admin/dashboard')
    expect(fetchMock.mock.calls[0]?.[1]).toEqual(
      expect.objectContaining({ credentials: 'include', method: 'GET' }),
    )
  })
})
