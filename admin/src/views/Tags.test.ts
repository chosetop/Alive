import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import Tags from './Tags.vue'

const api = vi.hoisted(() => ({
  listTags: vi.fn(),
  createTag: vi.fn(),
  updateTag: vi.fn(),
  deleteTag: vi.fn(),
}))

vi.mock('../api', () => ({
  tagsApi: {
    listTags: api.listTags,
    createTag: api.createTag,
    updateTag: api.updateTag,
    deleteTag: api.deleteTag,
  },
  toUserMessage: () => '出现了意外错误，请稍后重试。',
}))

const wrappers: VueWrapper[] = []

function mountTags(): VueWrapper {
  const wrapper = mount(Tags)
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  api.listTags.mockResolvedValue([
    { id: 1, name: '旅行', slug: 'travel', usage_count: 2 },
    { id: 2, name: '读书', slug: 'reading', usage_count: 0 },
  ])
  api.createTag.mockResolvedValue({ id: 3, name: '工作', slug: 'work', usage_count: 0 })
  api.updateTag.mockResolvedValue({ id: 2, name: '阅读', slug: 'reading', usage_count: 0 })
  api.deleteTag.mockResolvedValue(undefined)
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  vi.clearAllMocks()
})

describe('Tags', () => {
  it('loads tags and explains usage constraints', async () => {
    const wrapper = mountTags()
    await flushPromises()

    expect(api.listTags).toHaveBeenCalledWith({ q: undefined, limit: 50 })
    expect(wrapper.text()).toContain('旅行')
    expect(wrapper.text()).toContain('2 篇文章正在使用')
    expect(wrapper.get('button[title]').attributes('disabled')).toBeDefined()
  })

  it('creates and edits tags from the independent management page', async () => {
    const wrapper = mountTags()
    await flushPromises()

    await wrapper.get('[data-primary-action]').trigger('click')
    const createInputs = wrapper.findAll('[data-tag-form] input')
    await createInputs[0].setValue('工作')
    await createInputs[1].setValue('work')
    await wrapper.get('[data-tag-form]').trigger('submit')
    await flushPromises()
    expect(api.createTag).toHaveBeenCalledWith({ name: '工作', slug: 'work' })

    await wrapper.get('.row:nth-child(2) button').trigger('click')
    await wrapper.get('[data-tag-form] input').setValue('阅读')
    await wrapper.get('[data-tag-form]').trigger('submit')
    await flushPromises()
    expect(api.updateTag).toHaveBeenCalledWith(2, { name: '阅读' })
  })

  it('deletes an unused tag after confirmation', async () => {
    const wrapper = mountTags()
    await flushPromises()

    const deleteButton = wrapper.get('.row:nth-child(2) .row-actions button:last-child')
    await deleteButton.trigger('click')
    await wrapper.get('.row:nth-child(2) .row-actions button').trigger('click')
    await flushPromises()

    expect(api.deleteTag).toHaveBeenCalledWith(2)
  })
})
