import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { EntryListItem } from '../types/api'
import Entries from './Entries.vue'

const api = vi.hoisted(() => ({ listEntriesAdmin: vi.fn() }))

vi.mock('../api', () => ({
  entriesApi: { listEntriesAdmin: api.listEntriesAdmin },
  toUserMessage: () => '出现了意外错误，请稍后重试。',
}))

const wrappers: VueWrapper[] = []

function mountEntries(): VueWrapper {
  const wrapper = mount(Entries, {
    global: {
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

const entry: EntryListItem = {
  id: 7,
  type: 'journal',
  title: '待完成的文章',
  slug: 'draft-entry',
  summary: '',
  cover_url: '',
  meta: {},
  word_count: 0,
  category: null,
  happened_at: null,
  published_at: null,
  status: 'draft',
  visibility: 'public',
  created_at: '2026-08-27T00:00:00Z',
  updated_at: '2026-08-27T00:00:00Z',
}

beforeEach(() => {
  api.listEntriesAdmin.mockResolvedValue({
    data: [entry],
    meta: { page: 1, page_size: 20, total: 1 },
  })
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  vi.clearAllMocks()
})

describe('Entries', () => {
  it('keeps content actions and status readable as semantic controls', () => {
    const wrapper = mountEntries()

    // Removing these hooks would leave the heading and action visually styled
    // but make their page-level relationship impossible to target consistently.
    expect(wrapper.find('[data-page-header]').exists()).toBe(true)
    expect(wrapper.get('[role="tablist"]').attributes('aria-label')).toBe('按状态筛选')
    expect(wrapper.get('[data-primary-action]').text()).toBe('写一篇')
  })

  it('sends the selected status filter to the paginated API', async () => {
    const wrapper = mountEntries()
    await flushPromises()

    await wrapper.get('[role="tab"][aria-label="草稿"]').trigger('click')
    await flushPromises()

    expect(api.listEntriesAdmin).toHaveBeenLastCalledWith({ page: 1, status: 'draft' })
  })
})
