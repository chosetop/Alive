import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { Category, EntryDetail } from '../../types/api'
import ArticleSettings from './ArticleSettings.vue'

const entry: EntryDetail = {
  id: 42,
  revision: 3,
  world: 'journal',
  kind: '',
  type: 'journal',
  title: '一篇文章',
  slug: 'an-article',
  summary: '摘要',
  content_md: '# 正文',
  cover_url: '',
  meta: {},
  word_count: 2,
  category_id: 0,
  category: null,
  happened_at: null,
  published_at: null,
  status: 'draft',
  visibility: 'public',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-26T00:00:00Z',
}

const categories: Category[] = [
  {
    id: 7,
    world: 'journal',
    name: '随笔',
    slug: 'notes',
    description: '',
    sort_order: 0,
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
  },
]

describe('ArticleSettings', () => {
  it('uses the shared writing-panel action hierarchy', () => {
    const wrapper = mount(ArticleSettings, {
      props: { open: true, entry, categories },
    })

    expect(wrapper.get('[data-writing-panel="settings"]').element).toBeTruthy()
    expect(wrapper.get('[data-settings-close] .ui-icon').attributes('aria-hidden')).toBe('true')
    expect(wrapper.get('[data-settings-delete]').attributes('data-variant')).toBe('danger')
  })

  it('edits the title alongside the other article settings', async () => {
    const wrapper = mount(ArticleSettings, {
      props: { open: true, entry, categories },
    })

    await wrapper.get('#e-title').setValue('设置里的标题')

    expect(wrapper.emitted('update')).toEqual([[{ title: '设置里的标题' }]])
  })

  it('emits one partial update when a setting changes', async () => {
    const wrapper = mount(ArticleSettings, {
      props: { open: true, entry, categories },
    })

    await wrapper.get('#e-slug').setValue('new-slug')

    expect(wrapper.emitted('update')).toEqual([[{ slug: 'new-slug' }]])
  })

  it('does not render the legacy type control', () => {
    const wrapper = mount(ArticleSettings, {
      props: { open: true, entry, categories },
    })

    expect(wrapper.find('#e-type').exists()).toBe(false)
  })

  it('requires explicit confirmation before emitting delete', async () => {
    const onDelete = vi.fn()
    const wrapper = mount(ArticleSettings, {
      props: { open: true, entry, categories, onDelete },
    })

    await wrapper.get('[data-settings-delete]').trigger('click')
    expect(onDelete).not.toHaveBeenCalled()
    expect(wrapper.get('[data-delete-confirm]').text()).toContain('确认删除')

    await wrapper.get('[data-delete-confirm]').trigger('click')
    expect(onDelete).toHaveBeenCalledOnce()
  })

  it('closes on Escape', async () => {
    const wrapper = mount(ArticleSettings, {
      props: { open: true, entry, categories },
    })

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(wrapper.emitted('update:open')).toEqual([[false]])
  })
})
