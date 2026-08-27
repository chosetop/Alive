import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { EntryDetail } from '../../types/api'
import PublishPanel from './PublishPanel.vue'

const entry: EntryDetail = {
  id: 42,
  revision: 4,
  type: 'journal',
  title: '一篇文章',
  slug: 'an-article',
  summary: '',
  content_md: '# 正文',
  cover_url: '',
  meta: {},
  word_count: 2,
  category_id: 0,
  category: null,
  happened_at: null,
  published_at: null,
  status: 'draft',
  visibility: 'unlisted',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-26T00:00:00Z',
}

describe('PublishPanel', () => {
  it('disables publish while blockers remain', () => {
    const wrapper = mount(PublishPanel, {
      props: {
        open: true,
        entry,
        checks: {
          blockers: [{ field: 'content_md', label: '正文', kind: 'blocker' }],
          reminders: [],
        },
      },
    })

    expect(wrapper.get('[data-publish-confirm]').attributes('disabled')).toBeDefined()
  })

  it('lets the writer acknowledge reminders and publishes the revision', async () => {
    const wrapper = mount(PublishPanel, {
      props: {
        open: true,
        entry,
        checks: {
          blockers: [],
          reminders: [{ field: 'summary', label: '摘要', kind: 'reminder' }],
        },
      },
    })

    await wrapper.get('[data-reminder-summary]').setValue(true)
    expect(wrapper.get('[data-visibility-copy]').text()).toContain('不列出')

    await wrapper.get('[data-publish-confirm]').trigger('click')
    expect(wrapper.emitted('publish')).toEqual([[42, 4]])
  })
})
