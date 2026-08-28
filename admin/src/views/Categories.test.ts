import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import Categories from './Categories.vue'

const api = vi.hoisted(() => ({ listCategoriesAdmin: vi.fn() }))

vi.mock('../api', () => ({
  buildCategoryPatch: vi.fn(),
  categoriesApi: {
    listCategoriesAdmin: api.listCategoriesAdmin,
    createCategory: vi.fn(),
    updateCategory: vi.fn(),
    deleteCategory: vi.fn(),
  },
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

    await wrapper.get('[data-primary-action]').trigger('click')
    expect(wrapper.find('form').exists()).toBe(true)
  })
})
