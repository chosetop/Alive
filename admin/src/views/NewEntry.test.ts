import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import NewEntry from './NewEntry.vue'

vi.mock('../api', () => ({ worldsApi: { listAdminWorlds: vi.fn().mockResolvedValue([]) } }))

describe('NewEntry', () => {
  it('offers only worlds with registered editors', () => {
    const wrapper = mount(NewEntry, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })

    const links = wrapper.findAll('a')
    expect(links).toHaveLength(3)
    expect(links[0]?.text()).toContain('写日志')
    expect(links[0]?.attributes('data-to')).toContain('"name":"entry-new-world"')
    expect(links[0]?.attributes('data-to')).toContain('"world":"journal"')
    expect(wrapper.text()).toContain('片语')
    expect(wrapper.text()).toContain('影像')
  })
})
