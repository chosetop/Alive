import { describe, expect, it } from 'vitest'

import PopoverToggle from './PopoverToggle.vue'
import { renderComponent } from '~/test-support/render'

describe('PopoverToggle', () => {
  it('starts closed with a non-toggling trigger button', async () => {
    const html = await renderComponent(PopoverToggle, { label: '选择一个选项' })

    expect(html).toContain('aria-expanded="false"')
    expect(html).toContain('type="button"')
    expect(html).not.toContain('<details')
    expect(html).not.toContain('<summary')
  })
})
