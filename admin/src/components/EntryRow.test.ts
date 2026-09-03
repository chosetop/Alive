import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { EntryListItem } from '../types/api'
import EntryRow from './EntryRow.vue'

const entry: EntryListItem = {
  id: 7,
  world: 'journal',
  kind: '',
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

describe('EntryRow', () => {
  it('keeps the draft status as a named chip beside the edit destination', () => {
    const wrapper = mount(EntryRow, {
      props: { entry },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })

    // If a status becomes plain, unstructured text, it loses the stable visual
    // and semantic boundary the list uses to distinguish the authoring queue.
    expect(wrapper.get('[data-status="draft"]').text()).toBe('草稿')
    expect(wrapper.get('[data-body-edit]').find('svg').exists()).toBe(true)
  })

  it('opens quick settings from the row and keeps body editing separate', async () => {
    const wrapper = mount(EntryRow, {
      props: { entry },
      global: { stubs: { RouterLink: { props: ['to'], template: '<a data-body-edit><slot /></a>' } } },
    })

    await wrapper.get('[data-entry-settings]').trigger('click')

    expect(wrapper.emitted('settings')).toEqual([[entry]])
    expect(wrapper.get('[data-body-edit]').text()).toBe('编辑正文')
  })
})
