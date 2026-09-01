import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import { useWritingStore } from '../stores/writing'
import type { EntryDetail } from '../types/api'
import WorldEditorHost from './WorldEditorHost.vue'
import { worldTransitionKey } from '../composables/useWorldTransition'

const api = vi.hoisted(() => ({
  getEntry: vi.fn(),
}))

vi.mock('../api', async () => {
  const errors = await import('../api/errors')
  return {
    ...errors,
    entriesApi: {
      getEntry: api.getEntry,
    },
  }
})

vi.mock('./EntryEditor.vue', () => ({
  default: {
    name: 'EntryEditor',
    props: ['id', 'initialEntry'],
    template: '<div data-editor-kind="long-form" :data-entry-id="id" :data-world="initialEntry?.world" />',
  },
}))

vi.mock('./SayingEditor.vue', () => ({
  default: {
    name: 'SayingEditor',
    props: ['initialEntry'],
    template: '<div data-editor-kind="saying" :data-entry-id="initialEntry?.id" :data-world="initialEntry?.world" />',
  },
}))

const wrappers: VueWrapper[] = []
const transition = {
  dispose: vi.fn(),
  enterWorld: vi.fn(),
  enterWorkspace: vi.fn().mockResolvedValue(undefined),
  swapCanvas: vi.fn(async (_surface: Element, replace: () => Promise<void>) => {
    await replace()
  }),
}

function entry(overrides: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 41,
    revision: 1,
    world: 'journal',
    kind: '',
    type: 'journal',
    title: '原始标题',
    slug: 'original-title',
    summary: '',
    content_md: '服务端正文',
    cover_url: '',
    meta: {},
    word_count: 4,
    category_id: 0,
    category: null,
    happened_at: null,
    published_at: null,
    status: 'draft',
    visibility: 'public',
    created_at: '2026-08-20T00:00:00Z',
    updated_at: '2026-08-25T10:00:00Z',
    ...overrides,
  }
}

async function mountHost(id = '41'): Promise<VueWrapper> {
  const wrapper = mount(WorldEditorHost, {
    props: { id },
    global: {
      plugins: [createPinia()],
      provide: {
        [worldTransitionKey as symbol]: transition,
      },
    },
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('WorldEditorHost', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    setActivePinia(createPinia())
    transition.enterWorkspace.mockClear()
    transition.swapCanvas.mockClear()
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  })

  it('routes saying entries to the saying editor and records the active world', async () => {
    api.getEntry.mockResolvedValue(entry({ world: 'saying' }))

    const wrapper = await mountHost()

    expect(api.getEntry).toHaveBeenCalledExactlyOnceWith(41)
    expect(wrapper.find('[data-editor-kind="saying"]').exists()).toBe(true)
    expect(wrapper.find('[data-editor-kind="long-form"]').exists()).toBe(false)
    expect(useWritingStore().activeWorld).toBe('saying')
  })

  it('routes journal entries to the long-form editor with the preloaded entry', async () => {
    api.getEntry.mockResolvedValue(entry({ id: 42, world: 'journal' }))

    const wrapper = await mountHost('42')

    expect(wrapper.find('[data-editor-kind="saying"]').exists()).toBe(false)
    expect(wrapper.get('[data-editor-kind="long-form"]').attributes('data-entry-id')).toBe('42')
    expect(wrapper.get('[data-editor-kind="long-form"]').attributes('data-world')).toBe('journal')
    expect(useWritingStore().activeWorld).toBe('journal')
    expect(transition.enterWorkspace).toHaveBeenCalledOnce()
  })

  it('routes video entries to the long-form editor with the preloaded entry', async () => {
    api.getEntry.mockResolvedValue(entry({ id: 43, world: 'video' }))

    const wrapper = await mountHost('43')

    expect(wrapper.find('[data-editor-kind="saying"]').exists()).toBe(false)
    expect(wrapper.get('[data-editor-kind="long-form"]').attributes('data-entry-id')).toBe('43')
    expect(wrapper.get('[data-editor-kind="long-form"]').attributes('data-world')).toBe('video')
    expect(useWritingStore().activeWorld).toBe('video')
  })

  it('swaps the stable editor surface when navigating between two entry ids', async () => {
    api.getEntry
      .mockResolvedValueOnce(entry({ id: 41, world: 'journal' }))
      .mockResolvedValueOnce(entry({ id: 42, world: 'video' }))

    const wrapper = await mountHost('41')
    await wrapper.setProps({ id: '42' })
    await flushPromises()

    expect(api.getEntry).toHaveBeenNthCalledWith(1, 41)
    expect(api.getEntry).toHaveBeenNthCalledWith(2, 42)
    expect(transition.swapCanvas).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-editor-kind="long-form"]').attributes('data-entry-id')).toBe('42')
    expect(wrapper.get('[data-editor-kind="long-form"]').attributes('data-world')).toBe('video')
    expect(useWritingStore().activeWorld).toBe('video')
  })

  it('surfaces unsupported worlds as an alert and renders no editor', async () => {
    api.getEntry.mockResolvedValue(entry({ world: 'unknown' as EntryDetail['world'] }))

    const wrapper = await mountHost()

    expect(wrapper.find('[data-editor-kind]').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toContain('Unsupported world')
  })

  it('ignores a late entry load after the host unmounts', async () => {
    let resolveEntry: (value: EntryDetail) => void = () => {}
    api.getEntry.mockReturnValue(
      new Promise<EntryDetail>((resolve) => {
        resolveEntry = resolve
      }),
    )

    const wrapper = mount(WorldEditorHost, {
      props: { id: '41' },
      global: { plugins: [createPinia()] },
    })
    wrappers.push(wrapper)

    expect(useWritingStore().activeWorld).toBeNull()

    wrapper.unmount()
    wrappers.splice(wrappers.indexOf(wrapper), 1)
    resolveEntry(entry({ world: 'saying' }))
    await flushPromises()
    await nextTick()

    expect(useWritingStore().activeWorld).toBeNull()
  })
})
