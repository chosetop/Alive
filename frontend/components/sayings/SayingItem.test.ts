import { describe, expect, it } from 'vitest'

import SayingItem from './SayingItem.vue'
import { renderComponent } from '~/test-support/render'

describe('SayingItem', () => {
  it('renders the body without a time, title, or other journal chrome', async () => {
    const html = await renderComponent(SayingItem, {
      item: {
        short_id: 'abc123',
        content_md: '第一行\n\n**第二行**',
        source: '《旧句》',
        author: '某人',
        category: { id: 7, name: '摘句', slug: 'quotes' },
      },
    })

    expect(html).toContain('<strong>第二行</strong>')
    expect(html).toContain('《旧句》')
    expect(html).toContain('某人')
    expect(html).toContain('摘句')
    expect(html).not.toContain('<time')
    expect(html).not.toContain('<h1')
    expect(html).not.toContain('<h2')
    expect(html).not.toContain('title')
  })
})
