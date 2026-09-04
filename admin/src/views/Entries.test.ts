import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { EntryDetail, EntryListItem, PaginatedResponse } from '../types/api'
import Entries from './Entries.vue'

const api = vi.hoisted(() => ({
  entriesApi: {
    listEntriesAdmin: vi.fn(),
    getEntry: vi.fn(),
    updateEntry: vi.fn(),
    deleteEntry: vi.fn(),
  },
  categoriesApi: { listCategoriesAdmin: vi.fn() },
}))

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  entriesApi: api.entriesApi,
  categoriesApi: api.categoriesApi,
  toUserMessage: () => '请求失败',
}))

function page(): PaginatedResponse<EntryListItem> {
  return { data: [], meta: { page: 1, page_size: 20, total: 0 } }
}

async function render(
  query: Record<string, string> = {},
  response: PaginatedResponse<EntryListItem> = page(),
) {
  api.entriesApi.listEntriesAdmin.mockResolvedValue(response)
  api.categoriesApi.listCategoriesAdmin.mockResolvedValue([
    {
      id: 1,
      world: undefined,
      name: '旅行',
      slug: 'travel',
      description: '',
      created_at: '',
      updated_at: '',
    },
  ])

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/entries', name: 'entries', component: Entries },
      { path: '/entries/new', name: 'entry-new', component: { template: '<div />' } },
      { path: '/entries/:id', name: 'entry-edit', component: { template: '<div />' } },
    ],
  })
  await router.push({ path: '/entries', query })
  await router.isReady()
  const wrapper = mount(Entries, { global: { plugins: [router] } })
  await flushPromises()
  return { router, wrapper }
}

function entryDetail(): EntryDetail {
  return {
    id: 7,
    revision: 1,
    world: 'saying',
    kind: '',
    title: '很多小事',
    slug: 'paperday22',
    summary: '',
    content_md: '一句话',
    cover_url: '',
    meta: {},
    word_count: 4,
    category_id: 0,
    category: null,
    happened_at: null,
    published_at: null,
    status: 'published',
    visibility: 'public',
    tags: [],
    created_at: '2026-08-20T00:00:00Z',
    updated_at: '2026-08-27T00:00:00Z',
  }
}

describe('Entries filters', () => {
  beforeEach(() => {
    api.entriesApi.listEntriesAdmin.mockReset()
    api.categoriesApi.listCategoriesAdmin.mockReset()
    api.entriesApi.getEntry.mockReset()
    api.entriesApi.updateEntry.mockReset()
    api.entriesApi.deleteEntry.mockReset()
  })

  it('restores search and category from URL and sends them together', async () => {
    await render({ q: '山中', category: 'travel', status: 'draft' })

    expect(api.entriesApi.listEntriesAdmin).toHaveBeenLastCalledWith({
      page: 1,
      world: undefined,
      status: 'draft',
      q: '山中',
      category: 'travel',
    })
  })

  it('writes combined filter changes to the URL query', async () => {
    const { router, wrapper } = await render()

    await wrapper.get('[data-test="entry-search"]').setValue('mountain')
    await wrapper.get('[data-test="entry-category"]').setValue('travel')
    await flushPromises()

    expect(router.currentRoute.value.query).toMatchObject({ q: 'mountain', category: 'travel' })
    expect(api.entriesApi.listEntriesAdmin).toHaveBeenLastCalledWith({
      page: 1,
      world: undefined,
      q: 'mountain',
      category: 'travel',
      status: undefined,
    })
  })


  it('opens the existing article settings drawer for a list row', async () => {
    const detail = entryDetail()
    const response: PaginatedResponse<EntryListItem> = {
      data: [detail],
      meta: { page: 1, page_size: 20, total: 1 },
    }
    api.entriesApi.getEntry.mockResolvedValue(detail)
    const { wrapper } = await render({}, response)

    await wrapper.get('[data-entry-settings]').trigger('click')
    await flushPromises()

    expect(api.entriesApi.getEntry).toHaveBeenCalledWith(7)
    expect(wrapper.get('[data-writing-panel="settings"]').text()).toContain('文章设置')
  })

  it('saves changes made in the quick-settings drawer', async () => {
    const detail = entryDetail()
    const response: PaginatedResponse<EntryListItem> = {
      data: [detail],
      meta: { page: 1, page_size: 20, total: 1 },
    }
    api.entriesApi.getEntry.mockResolvedValue(detail)
    api.entriesApi.updateEntry.mockResolvedValue({ ...detail, title: '新标题', revision: 2 })
    const { wrapper } = await render({}, response)

    await wrapper.get('[data-entry-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('#e-title').setValue('新标题')
    await wrapper.get('[data-settings-save]').trigger('click')
    await flushPromises()

    expect(api.entriesApi.updateEntry).toHaveBeenCalledWith(7, {
      revision: 1,
      title: '新标题',
    })
    expect(wrapper.get('[data-entry-settings]').text()).toContain('一句话')
  })
})
