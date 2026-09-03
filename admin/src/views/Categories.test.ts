import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import Categories from './Categories.vue'

const api = vi.hoisted(() => ({
  listCategoriesAdmin: vi.fn(),
  createCategory: vi.fn(),
  updateCategory: vi.fn(),
  deleteCategory: vi.fn(),
  listAdminWorlds: vi.fn(),
}))

vi.mock('../api', () => ({
  buildCategoryPatch: vi.fn(),
  categoriesApi: {
    listCategoriesAdmin: api.listCategoriesAdmin,
    createCategory: api.createCategory,
    updateCategory: api.updateCategory,
    deleteCategory: api.deleteCategory,
  },
  worldsApi: { listAdminWorlds: api.listAdminWorlds },
  isEmptyPatch: vi.fn(),
  toUserMessage: () => '出现了意外错误，请稍后重试。',
}))

const wrappers: VueWrapper[] = []

function mountCategories(): VueWrapper {
  const wrapper = mount(Categories)
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  api.listCategoriesAdmin.mockResolvedValue([])
  api.listAdminWorlds.mockResolvedValue([{ world: 'journal', status: 'open' }])
  api.createCategory.mockResolvedValue({
    id: 1,
    world: 'journal',
    name: '旅行',
    slug: 'travel',
    description: '',
    sort_order: 0,
    created_at: '2026-08-29T00:00:00Z',
    updated_at: '2026-08-29T00:00:00Z',
  })
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  vi.clearAllMocks()
})

describe('Categories', () => {
  it('keeps the create action in the semantic page header', async () => {
    const wrapper = mountCategories()
    await flushPromises()

    // The new-category action must remain a real control associated with the
    // header, rather than becoming a decorative card link.
    expect(wrapper.find('[data-page-header]').exists()).toBe(true)
    expect(wrapper.get('[data-primary-action]').text()).toBe('新建分类')
    expect(wrapper.find('.subtitle').exists()).toBe(false)
    expect(wrapper.find('[data-world-status]').exists()).toBe(false)
    expect(wrapper.get('[data-world-filter]').attributes('role')).toBe('group')
    expect(wrapper.find('[data-world-filter] select').exists()).toBe(false)

    await wrapper.get('[data-primary-action]').trigger('click')
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('loads and creates categories inside the selected world', async () => {
    const wrapper = mountCategories()
    await flushPromises()

    expect(api.listCategoriesAdmin).toHaveBeenCalledWith({ world: 'journal' })

    await wrapper.get('[data-primary-action]').trigger('click')
    await wrapper.get('#cat-name').setValue('旅行')
    await wrapper.get('#cat-slug').setValue('travel')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.createCategory).toHaveBeenCalledWith({
      world: 'journal',
      name: '旅行',
      slug: 'travel',
      description: '',
      sort_order: 0,
    })
  })

  it('switches worlds with three separate segmented buttons', async () => {
    const wrapper = mountCategories()
    await flushPromises()

    const options = wrapper.findAll('[data-world-option]')
    expect(options.map((option) => option.text())).toEqual(['日志', '片语', '影像'])
    expect(wrapper.get('[data-world-option="journal"]').attributes('aria-pressed')).toBe('true')

    await wrapper.get('[data-world-option="saying"]').trigger('click')
    await flushPromises()

    expect(api.listCategoriesAdmin).toHaveBeenLastCalledWith({ world: 'saying' })
    expect(wrapper.get('[data-world-option="saying"]').attributes('aria-pressed')).toBe('true')
  })
})
