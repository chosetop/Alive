import { describe, expect, it } from 'vitest'

import SayingsHomePreview from './SayingsHomePreview.vue'
import { renderComponent } from '~/test-support/render'

describe('SayingsHomePreview', () => {
  it('shows a fixed three-item preview and the browse-all link', async () => {
    const html = await renderComponent(SayingsHomePreview, {
      items: [
        { short_id: 'a1', content_md: '一' },
        { short_id: 'b2', content_md: '二' },
        { short_id: 'c3', content_md: '三' },
        { short_id: 'd4', content_md: '四' },
      ],
    })

    expect(html).toContain('/sayings')
    expect(html.match(/class="item"/g)).toHaveLength(3)
    expect(html).toContain('一')
    expect(html).toContain('二')
    expect(html).toContain('三')
  })
})
