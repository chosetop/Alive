import { describe, expect, it } from 'vitest'

import SayingViewPicker from './SayingViewPicker.vue'
import { renderComponent } from '~/test-support/render'

describe('SayingViewPicker', () => {
  it('marks the selected view and exposes all three options', async () => {
    const html = await renderComponent(SayingViewPicker, { view: 'wall' })

    expect(html).toContain('aria-expanded="false"')
    expect(html).toContain('纸片墙')
    expect(html).not.toContain('role="menu"')
  })
})
