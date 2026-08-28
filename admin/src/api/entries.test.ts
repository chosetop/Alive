import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { listEntriesAdmin } from './entries'

/**
 * The admin list's query serialisation.
 *
 * These assert on the URL that leaves the client, because that is the contract
 * with the backend handler: `q` present means search, `q` absent means no text
 * filter. The distinction matters for the directory search field, which sends a
 * value on every keystroke including the one that clears it.
 */
describe('listEntriesAdmin', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [], meta: { page: 1, page_size: 20, total: 0 } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  /** The URL of the single request the call under test made. */
  function requestedUrl(): URL {
    expect(fetchMock).toHaveBeenCalledTimes(1)
    return new URL(String(fetchMock.mock.calls[0]?.[0]))
  }

  it('sends q when a search term is given', async () => {
    await listEntriesAdmin({ q: 'mountain' })

    expect(requestedUrl().searchParams.get('q')).toBe('mountain')
  })

  it('sends q alongside status, so the two filters intersect', async () => {
    await listEntriesAdmin({ q: 'mountain', status: 'draft' })

    const params = requestedUrl().searchParams
    expect(params.get('q')).toBe('mountain')
    expect(params.get('status')).toBe('draft')
  })

  it('sends category alongside q and status, so all filters intersect', async () => {
    await listEntriesAdmin({ q: 'mountain', status: 'draft', category: 'travel' })

    const params = requestedUrl().searchParams
    expect(params.get('q')).toBe('mountain')
    expect(params.get('status')).toBe('draft')
    expect(params.get('category')).toBe('travel')
  })

  it('omits q entirely when no search term is given', async () => {
    await listEntriesAdmin({ page: 1 })

    // Not `q=undefined`, which the backend would receive as a literal search for
    // the string "undefined" and answer with an empty directory.
    expect(requestedUrl().searchParams.has('q')).toBe(false)
  })

  it('preserves a query with CJK characters', async () => {
    await listEntriesAdmin({ q: '京都' })

    expect(requestedUrl().searchParams.get('q')).toBe('京都')
  })
})
