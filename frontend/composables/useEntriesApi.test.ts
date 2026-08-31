import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  getPage: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  requestNoContent: vi.fn(),
}))

vi.mock('./useApi', () => ({
  useApi: () => api,
}))

import { useEntriesApi } from './useEntriesApi'

describe('useEntriesApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('routes the public list through /journals', async () => {
    api.getPage.mockResolvedValue({ data: [], meta: { page: 1, page_size: 20, total: 0 } })

    await useEntriesApi().list({ page: 2, category: 'travel' })

    expect(api.getPage).toHaveBeenCalledWith('/journals', { page: 2, category: 'travel' })
  })

  it('routes public detail lookups through /journals/:slug', async () => {
    api.get.mockResolvedValue({ slug: 'spring-river' })

    await useEntriesApi().getBySlug('spring-river')

    expect(api.get).toHaveBeenCalledWith('/journals/spring-river')
  })
})
