import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiClientError } from '../api/errors'
import Worlds from './Worlds.vue'

const api = vi.hoisted(() => ({
  listAdminWorlds: vi.fn(),
  updateWorld: vi.fn(),
}))

vi.mock('../api', async () => {
  const errors = await import('../api/errors')
  return {
    ...errors,
    worldsApi: {
      listAdminWorlds: api.listAdminWorlds,
      updateWorld: api.updateWorld,
    },
  }
})

const wrappers: VueWrapper[] = []

function settings() {
  return [
    {
      world: 'journal',
      status: 'open',
      nav_label: '日志',
      sort_order: 10,
      default_view: '',
      revision: 1,
      updated_at: '2026-08-29T09:00:00Z',
    },
    {
      world: 'saying',
      status: 'unopened',
      nav_label: '片语',
      sort_order: 20,
      default_view: 'stream',
      revision: 2,
      updated_at: '2026-08-29T09:00:00Z',
    },
    {
      world: 'video',
      status: 'unopened',
      nav_label: '影像',
      sort_order: 30,
      default_view: '',
      revision: 1,
      updated_at: '2026-08-29T09:00:00Z',
    },
  ] as const
}

function mountWorlds(): VueWrapper {
  const wrapper = mount(Worlds)
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  vi.resetAllMocks()
  api.listAdminWorlds.mockResolvedValue(settings())
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
})

describe('Worlds', () => {
  it('renders the supported worlds in fixed order', async () => {
    const wrapper = mountWorlds()
    await flushPromises()

    expect(wrapper.findAll('[data-world-row]').map((row) => row.attributes('data-world-key'))).toEqual([
      'journal',
      'saying',
      'video',
    ])
  })

  it('reloads the stale row and asks for another save after a revision conflict', async () => {
    api.updateWorld.mockRejectedValueOnce(
      new ApiClientError({
        code: 'CONFLICT',
        status: 409,
        message: 'stale revision',
        fields: { revision: 'please reload' },
      }),
    )
    api.listAdminWorlds
      .mockResolvedValueOnce(settings())
      .mockResolvedValueOnce([
        settings()[0],
        {
          ...settings()[1],
          status: 'open',
          revision: 3,
        },
        settings()[2],
      ])

    const wrapper = mountWorlds()
    await flushPromises()

    await wrapper.get('[data-world-status="saying"]').setValue('open')
    await wrapper.get('[data-world-save="saying"]').trigger('click')
    await flushPromises()

    expect(api.updateWorld).toHaveBeenCalledWith('saying', {
      revision: 2,
      status: 'open',
      nav_label: '片语',
      default_view: 'stream',
    })
    expect(api.listAdminWorlds).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[role="alert"]').text()).toContain('请先重新确认')
  })
})
