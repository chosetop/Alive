import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiClientError } from '../../api/errors'
import { ADMIN_WORLD_REGISTRY } from '../../content-worlds/registry'
import type { AdminWorldSetting } from '../../types/api'
import WorldSettingsPanel from './WorldSettingsPanel.vue'
import { UiSelect } from '../ui'

const api = vi.hoisted(() => ({
  updateWorld: vi.fn(),
}))

vi.mock('../../api', async () => {
  const errors = await import('../../api/errors')
  return {
    ...errors,
    worldsApi: {
      updateWorld: api.updateWorld,
    },
  }
})

const wrappers: VueWrapper[] = []

function setting(world: 'journal' | 'saying' | 'video'): AdminWorldSetting {
  const base: Record<typeof world, AdminWorldSetting> = {
    journal: {
      world: 'journal',
      status: 'open',
      nav_label: '日志',
      sort_order: 10,
      default_view: '',
      revision: 1,
      updated_at: '2026-08-29T09:00:00Z',
    },
    saying: {
      world: 'saying',
      status: 'open',
      nav_label: '片语',
      sort_order: 20,
      default_view: 'stream',
      revision: 2,
      updated_at: '2026-08-29T09:00:00Z',
    },
    video: {
      world: 'video',
      status: 'hidden',
      nav_label: '影像',
      sort_order: 30,
      default_view: '',
      revision: 3,
      updated_at: '2026-08-29T09:00:00Z',
    },
  }
  return base[world]
}

function mountPanel(world: 'journal' | 'saying' | 'video', reload = vi.fn()): VueWrapper {
  const wrapper = mount(WorldSettingsPanel, {
    props: {
      definition: ADMIN_WORLD_REGISTRY.find((item) => item.key === world)!,
      setting: setting(world),
      open: true,
      reload,
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  vi.resetAllMocks()
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
})

describe('WorldSettingsPanel', () => {
  it('renders the default-view selector only for the saying world', () => {
    const journal = mountPanel('journal')
    const saying = mountPanel('saying')
    const video = mountPanel('video')

    expect(journal.find('[data-world-default-view]').exists()).toBe(false)
    expect(saying.find('[data-world-default-view]').exists()).toBe(true)
    expect(video.find('[data-world-default-view]').exists()).toBe(false)
  })

  it('keeps save disabled until the local draft is dirty and emits the saved payload', async () => {
    api.updateWorld.mockResolvedValueOnce({
      ...setting('saying'),
      status: 'hidden',
      revision: 3,
      updated_at: '2026-08-31T08:00:00Z',
    })

    const wrapper = mountPanel('saying')

    expect(wrapper.get('[data-world-save]').attributes('disabled')).toBeDefined()

    wrapper.findAllComponents(UiSelect).find((select) => select.props('label') === '状态')?.vm.$emit('change', 'hidden')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-world-save]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-world-save]').trigger('click')
    await flushPromises()

    expect(api.updateWorld).toHaveBeenCalledWith('saying', {
      revision: 2,
      status: 'hidden',
      nav_label: '片语',
      default_view: 'stream',
    })
    expect(wrapper.emitted('saved')).toEqual([[expect.objectContaining({ world: 'saying', revision: 3 })]])
  })

  it('reloads on a revision conflict without emitting a stale saved payload', async () => {
    const reload = vi.fn().mockResolvedValue({
      ...setting('saying'),
      status: 'hidden',
      revision: 3,
    })
    api.updateWorld.mockRejectedValueOnce(
      new ApiClientError({
        code: 'CONFLICT',
        status: 409,
        message: 'stale revision',
        fields: { revision: 'reload' },
      }),
    )

    const wrapper = mountPanel('saying', reload)

    wrapper.findAllComponents(UiSelect).find((select) => select.props('label') === '状态')?.vm.$emit('change', 'hidden')
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-world-save]').trigger('click')
    await flushPromises()

    expect(reload).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('设置已被其他保存更新，请重新确认。')
  })
})
