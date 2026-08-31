import { describe, expect, it } from 'vitest'

import SayingViewPicker from './SayingViewPicker.vue'
import { renderComponent } from '~/test-support/render'

describe('SayingViewPicker', () => {
  it('marks the selected view and exposes all three options', async () => {
    const html = await renderComponent(SayingViewPicker, { view: 'wall' })

    expect(html).toContain('role="radiogroup"')
    expect(html).toContain('流式')
    expect(html).toContain('纸片墙')
    expect(html).toContain('沉浸')
    expect(html).toContain('aria-checked="true"')
    expect(html).toContain('aria-checked="false"')
  })
})
