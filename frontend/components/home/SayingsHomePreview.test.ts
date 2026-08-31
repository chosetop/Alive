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
    expect(html).toContain('href="/sayings/a1"')
    expect(html).toContain('href="/sayings/b2"')
    expect(html).toContain('href="/sayings/c3"')
    expect(html).not.toContain('href="/sayings/d4"')
    expect(html).toContain('一')
    expect(html).toContain('二')
    expect(html).toContain('三')
  })
})
