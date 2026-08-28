import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import Dashboard from './Dashboard.vue'
import { useAuthStore } from '../stores/auth'

const getDashboardMetrics = vi.hoisted(() => vi.fn())

vi.mock('../api/dashboard', () => ({ getDashboardMetrics }))

describe('Dashboard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.user = {
      id: 1,
      username: 'owner',
      role: 'owner',
      display_name: 'Alive',
    }
    getDashboardMetrics.mockResolvedValue({
      total_entries: 12,
      published_entries: 8,
      total_words: 3456,
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows live writing metrics instead of the editor placeholder', async () => {
    const wrapper = mount(Dashboard, {
      global: {
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await nextTick()
    await nextTick()

    expect(getDashboardMetrics).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('已写字数')
    expect(wrapper.text()).toContain('3,456')
    expect(wrapper.text()).toContain('总文章数')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('已发布')
    expect(wrapper.text()).toContain('8')
    expect(wrapper.text()).not.toContain('编辑器将在下一步接入')
  })
})
