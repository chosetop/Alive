import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Entries from './Entries.vue'

const api = vi.hoisted(() => ({
  entriesApi: { listEntriesAdmin: vi.fn() },
  categoriesApi: { listCategoriesAdmin: vi.fn() },
}))

vi.mock('../api', () => ({
  entriesApi: api.entriesApi,
  categoriesApi: api.categoriesApi,
  toUserMessage: () => '请求失败',
}))

function page() {
  return { data: [], meta: { page: 1, page_size: 20, total: 0 } }
}

async function render(query: Record<string, string> = {}) {
  api.entriesApi.listEntriesAdmin.mockResolvedValue(page())
  api.categoriesApi.listCategoriesAdmin.mockResolvedValue([
    { id: 1, name: '旅行', slug: 'travel', description: '', created_at: '', updated_at: '' },
  ])

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/entries', name: 'entries', component: Entries },
      { path: '/entries/new', name: 'entry-new', component: { template: '<div />' } },
    ],
  })
  await router.push({ path: '/entries', query })
  await router.isReady()
  const wrapper = mount(Entries, { global: { plugins: [router] } })
  await flushPromises()
  return { router, wrapper }
}

describe('Entries filters', () => {
  beforeEach(() => {
    api.entriesApi.listEntriesAdmin.mockReset()
    api.categoriesApi.listCategoriesAdmin.mockReset()
  })

  it('restores search and category from URL and sends them together', async () => {
    await render({ q: '山中', category: 'travel', status: 'draft' })

    expect(api.entriesApi.listEntriesAdmin).toHaveBeenLastCalledWith({
      page: 1,
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
      q: 'mountain',
      category: 'travel',
    })
  })
})
