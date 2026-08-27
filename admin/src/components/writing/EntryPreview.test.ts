import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import EntryPreview from './EntryPreview.vue'

describe('EntryPreview', () => {
  it('renders the local title and markdown output', () => {
    const wrapper = mount(EntryPreview, {
      props: { open: true, title: '本地标题', contentMd: '**正文**' },
    })

    expect(wrapper.get('[data-preview-title]').text()).toBe('本地标题')
    expect(wrapper.get('[data-preview-content] strong').text()).toBe('正文')
  })

  it('switches between desktop and mobile preview widths', async () => {
    const wrapper = mount(EntryPreview, {
      props: { open: true, title: '标题', contentMd: '正文' },
    })

    await wrapper.get('[data-preview-mobile]').trigger('click')

    expect(wrapper.get('[data-preview-frame]').classes()).toContain('is-mobile')
  })
})
