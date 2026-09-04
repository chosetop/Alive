import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useCategoriesApi } from './useCategoriesApi'

const api = vi.hoisted(() => ({
  getList: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  requestNoContent: vi.fn(),
}))

vi.mock('./useApi', () => ({
  useApi: () => api,
}))

describe('useCategoriesApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sends the journal world filter with public category requests', async () => {
    api.getList.mockResolvedValue({ data: [] })

    await useCategoriesApi().list({ world: 'journal' })

    expect(api.getList).toHaveBeenCalledWith('/categories', { world: 'journal' })
  })
})
