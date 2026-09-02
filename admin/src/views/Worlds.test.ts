import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiClientError } from '../api/errors'
import Worlds from './Worlds.vue'

const api = vi.hoisted(() => ({
  listAdminWorlds: vi.fn(),
  updateWorld: vi.fn(),
  listEntriesAdmin: vi.fn(),
  getEntry: vi.fn(),
  listCategoriesAdmin: vi.fn(),
}))

const navigation = vi.hoisted(() => ({
  push: vi.fn(),
}))

const transition = vi.hoisted(() => ({
  dispose: vi.fn(),
  enterWorld: vi.fn((_: Element, __: Element[], navigate: () => Promise<void>) => navigate()),
  enterWorkspace: vi.fn(),
  swapCanvas: vi.fn(),
}))

vi.mock('../api', async () => {
  const errors = await import('../api/errors')
  return {
    ...errors,
    worldsApi: {
      listAdminWorlds: api.listAdminWorlds,
      updateWorld: api.updateWorld,
    },
    entriesApi: {
      listEntriesAdmin: api.listEntriesAdmin,
      getEntry: api.getEntry,
    },
    categoriesApi: { listCategoriesAdmin: api.listCategoriesAdmin },
  }
})

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return { ...actual, useRouter: () => ({ push: navigation.push }) }
})

vi.mock('../composables/useWorldTransition', () => ({
  useWorldTransition: () => transition,
  worldTransitionKey: Symbol('world-transition'),
  writingRailKey: Symbol('writing-rail'),
}))

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
  api.listEntriesAdmin.mockImplementation(async ({ world }: { world: 'journal' | 'saying' | 'video' }) => ({
    data: world === 'journal'
      ? [{
          id: 7,
          world: 'journal',
          kind: 'entry',
          title: '一场缓慢的夏雨',
          slug: 'summer-rain',
          summary: '傍晚后的雷声没有追上雨。',
          cover_url: '',
          meta: {},
          word_count: 728,
          category: null,
          happened_at: '2026-08-12T10:00:00Z',
          published_at: null,
          status: 'draft',
          visibility: 'private',
          created_at: '2026-08-12T10:00:00Z',
          updated_at: '2026-08-30T08:00:00Z',
        }]
      : [],
    meta: { page: 1, page_size: 1, total: world === 'journal' ? 1 : 0 },
  }))
  api.listCategoriesAdmin.mockResolvedValue([])
  api.getEntry.mockResolvedValue({
    id: 7,
    world: 'journal',
    kind: 'entry',
    title: '一场缓慢的夏雨',
    slug: 'summer-rain',
    summary: '傍晚后的雷声没有追上雨。',
    cover_url: '',
    meta: {},
    word_count: 728,
    category: null,
    happened_at: '2026-08-12T10:00:00Z',
    published_at: null,
    status: 'draft',
    visibility: 'private',
    created_at: '2026-08-12T10:00:00Z',
    updated_at: '2026-08-30T08:00:00Z',
    revision: 3,
    content_md: '雨落在窗台。\n\n屋里的人没有急着关窗。',
    category_id: 0,
    tags: [],
  })
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
})

describe('Worlds', () => {
  it('uses the desktop main-desk layout and collapses to one column at 48rem', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'Worlds.vue'), 'utf8')

    expect(source).toMatch(/\.list\s*\{[\s\S]*grid-template-columns:\s*minmax\(0,\s*1\.25fr\)\s+minmax\(0,\s*0\.9fr\)/)
    expect(source).toMatch(/\.list\s*>\s*\[data-world-desk='journal'\]\s*\{[\s\S]*grid-row:\s*1\s*\/\s*span\s*2/)
    expect(source).toMatch(/@media \(max-width:\s*48rem\)[\s\S]*grid-template-columns:\s*1fr/)
  })

  it('renders the supported worlds in fixed order and requests one recent entry per world', async () => {
    const wrapper = mountWorlds()
    await flushPromises()

    expect(wrapper.findAll('[data-world-desk]').map((row) => row.attributes('data-world-desk'))).toEqual([
      'journal',
      'saying',
      'video',
    ])
    expect(api.listEntriesAdmin).toHaveBeenCalledWith({ world: 'journal', page_size: 1 })
    expect(api.listEntriesAdmin).toHaveBeenCalledWith({ world: 'saying', page_size: 1 })
    expect(api.listEntriesAdmin).toHaveBeenCalledWith({ world: 'video', page_size: 1 })
  })

  it('loads and renders the recent journal body without world descriptions', async () => {
    const wrapper = mountWorlds()
    await flushPromises()

    expect(api.getEntry).toHaveBeenCalledOnce()
    expect(api.getEntry).toHaveBeenCalledWith(7)
    expect(wrapper.get('[data-world-desk="journal"] [data-world-preview]').text()).toContain(
      '屋里的人没有急着关窗。',
    )
    expect(wrapper.text()).not.toContain('长文、图片与时间留下的痕迹')
    expect(wrapper.text()).not.toContain('没有时间轴的短句、片语与轻量记录。')
    expect(wrapper.text()).not.toContain('主视频、封面与说明。')
  })

  it('falls back to the recent journal summary when the detail preview fails', async () => {
    api.getEntry.mockRejectedValueOnce(new Error('detail unavailable'))

    const wrapper = mountWorlds()
    await flushPromises()

    expect(api.getEntry).toHaveBeenCalledWith(7)
    expect(wrapper.find('[data-world-desk="journal"] [data-world-preview]').exists()).toBe(false)
    expect(wrapper.get('[data-world-desk="journal"]').text()).toContain('傍晚后的雷声没有追上雨。')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('navigates to the recent entry when entering a populated world', async () => {
    const wrapper = mountWorlds()
    await flushPromises()

    await wrapper.get('[data-world-desk="journal"] [data-world-enter]').trigger('click')

    expect(transition.enterWorld).toHaveBeenCalledWith(
      wrapper.get('[data-world-desk="journal"]').element,
      expect.arrayContaining([
        wrapper.get('[data-world-desk="saying"]').element,
        wrapper.get('[data-world-desk="video"]').element,
      ]),
      expect.any(Function),
    )
    expect(navigation.push).toHaveBeenCalledWith({ name: 'entry-edit', params: { id: '7' } })
  })

  it('navigates to the world editor route when the desk is empty', async () => {
    const wrapper = mountWorlds()
    await flushPromises()

    await wrapper.get('[data-world-desk="video"] [data-world-enter]').trigger('click')

    expect(navigation.push).toHaveBeenCalledWith({ name: 'video-editor-new', params: { world: 'video' } })
  })

  it('keeps other desks visible when one snapshot request fails', async () => {
    api.listEntriesAdmin.mockImplementationOnce(async () => {
      throw new Error('entry list unavailable')
    })

    const wrapper = mountWorlds()
    await flushPromises()

    expect(wrapper.findAll('[data-world-desk]')).toHaveLength(3)
    expect(wrapper.get('[data-world-desk="journal"]').text()).toContain('暂时无法读取')
    await wrapper.get('[data-world-desk="journal"] [data-world-enter]').trigger('click')
    expect(navigation.push).not.toHaveBeenCalled()
  })

  it('retries a failed recent-entry snapshot before allowing entry', async () => {
    let journalAttempts = 0
    api.listEntriesAdmin.mockImplementation(async ({ world }: { world: 'journal' | 'saying' | 'video' }) => {
      if (world === 'journal') {
        journalAttempts += 1
        if (journalAttempts === 1) throw new Error('entry list unavailable')
        return {
          data: [{
            id: 9,
            world: 'journal',
            kind: 'entry',
            title: '重试后的最近编辑',
            slug: 'retried-entry',
            summary: '重试成功后才允许进入。',
            cover_url: '',
            meta: {},
            word_count: 320,
            category: null,
            happened_at: '2026-08-12T10:00:00Z',
            published_at: null,
            status: 'draft',
            visibility: 'private',
            created_at: '2026-08-12T10:00:00Z',
            updated_at: '2026-08-30T08:00:00Z',
          }],
          meta: { page: 1, page_size: 1, total: 1 },
        }
      }

      return {
        data: [],
        meta: { page: 1, page_size: 1, total: 0 },
      }
    })

    const wrapper = mountWorlds()
    await flushPromises()

    await wrapper.get('[data-world-desk="journal"] [data-world-retry]').trigger('click')
    await flushPromises()

    expect(api.listEntriesAdmin).toHaveBeenCalledTimes(4)
    expect(wrapper.get('[data-world-desk="journal"]').text()).toContain('重试后的最近编辑')

    await wrapper.get('[data-world-desk="journal"] [data-world-enter]').trigger('click')
    expect(navigation.push).toHaveBeenCalledWith({ name: 'entry-edit', params: { id: '9' } })
  })

  it('opens the matching settings panel and refreshes the saved desk snapshot', async () => {
    api.updateWorld.mockResolvedValueOnce({
      ...settings()[1],
      status: 'open',
      revision: 3,
      updated_at: '2026-08-31T08:00:00Z',
    })

    const wrapper = mountWorlds()
    await flushPromises()

    await wrapper.get('[data-world-desk="saying"] [data-world-settings]').trigger('click')
    expect(wrapper.get('[data-world-settings-panel]').attributes('data-world-key')).toBe('saying')

    await wrapper.get('[data-world-settings-panel] [data-world-status]').setValue('open')
    await wrapper.get('[data-world-settings-panel] [data-world-save]').trigger('click')
    await flushPromises()

    expect(api.updateWorld).toHaveBeenCalledWith('saying', {
      revision: 2,
      status: 'open',
      nav_label: '片语',
      default_view: 'stream',
    })
    expect(wrapper.get('[data-world-desk="saying"]').text()).toContain('已开放')
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

    await wrapper.get('[data-world-desk="saying"] [data-world-settings]').trigger('click')
    await wrapper.get('[data-world-settings-panel] [data-world-status]').setValue('open')
    await wrapper.get('[data-world-settings-panel] [data-world-save]').trigger('click')
    await flushPromises()

    expect(api.listAdminWorlds).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[role="alert"]').text()).toContain('请重新确认')
  })

  it('disposes the scoped transition if the page unmounts during a running world entry', async () => {
    let release = () => {}
    transition.enterWorld.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          release = () => resolve()
        }),
    )

    const wrapper = mountWorlds()
    await flushPromises()

    await wrapper.get('[data-world-desk="journal"] [data-world-enter]').trigger('click')
    wrapper.unmount()
    wrappers.splice(wrappers.indexOf(wrapper), 1)
    release()
    await flushPromises()

    expect(transition.dispose).toHaveBeenCalledOnce()
  })
})
