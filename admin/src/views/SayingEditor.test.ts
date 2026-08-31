import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref, type Ref } from 'vue'

import { useWritingStore, writingFlushKey } from '../stores/writing'
import type { EntryDetail } from '../types/api'
import SayingEditor from './SayingEditor.vue'

const api = vi.hoisted(() => ({
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  publishEntry: vi.fn(),
}))

const navigation = vi.hoisted(() => ({
  push: vi.fn(),
  leaveGuard: null as null | (() => Promise<boolean | void>),
  updateGuard: null as null | (() => Promise<boolean | void>),
}))

vi.mock('../api', async () => {
  const errors = await import('../api/errors')
  return {
    ...errors,
    entriesApi: {
      createEntry: api.createEntry,
      updateEntry: api.updateEntry,
      publishEntry: api.publishEntry,
    },
  }
})

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRouter: () => ({ push: navigation.push }),
    onBeforeRouteLeave: (guard: () => Promise<boolean | void>) => {
      navigation.leaveGuard = guard
    },
    onBeforeRouteUpdate: (guard: () => Promise<boolean | void>) => {
      navigation.updateGuard = guard
    },
  }
})

vi.mock('../components/writing/TagPicker.vue', () => ({
  default: {
    name: 'TagPicker',
    template: '<div data-tag-picker />',
  },
}))

const wrappers: VueWrapper[] = []

function entry(overrides: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 71,
    revision: 3,
    world: 'saying',
    kind: '',
    type: 'saying',
    title: '',
    slug: 'saying-entry',
    summary: '',
    content_md: '原句',
    cover_url: '',
    meta: { source: '摘录', author: '某人' },
    word_count: 2,
    category_id: 0,
    category: null,
    happened_at: null,
    published_at: null,
    status: 'draft',
    visibility: 'public',
    created_at: '2026-08-20T00:00:00Z',
    updated_at: '2026-08-25T10:00:00Z',
    tags: [],
    ...overrides,
  }
}

async function mountEditor(
  current: EntryDetail,
  flushGate?: Ref<(() => Promise<void>) | null>,
): Promise<VueWrapper> {
  const wrapper = mount(SayingEditor, {
    props: { initialEntry: current },
    global: {
      provide: flushGate === undefined ? {} : { [writingFlushKey as symbol]: flushGate },
      plugins: [createPinia()],
    },
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('SayingEditor', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    navigation.leaveGuard = null
    navigation.updateGuard = null
    api.createEntry.mockResolvedValue(entry({ id: 90, revision: 1, content_md: '' }))
    api.updateEntry.mockImplementation(async (_id: number, body: Record<string, unknown>) =>
      entry({
        revision: Number(body.revision) + 1,
        content_md: String(body.content_md ?? ''),
        visibility: (body.visibility as EntryDetail['visibility']) ?? 'public',
        meta: (body.meta as EntryDetail['meta']) ?? {},
      }),
    )
    api.publishEntry.mockResolvedValue(entry({ status: 'published', revision: 4 }))
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  })

  it('records the active saying entry on mount', async () => {
    const wrapper = await mountEditor(entry({ id: 72 }))
    const store = useWritingStore(wrapper.vm.$.appContext.config.globalProperties.$pinia)

    expect(store.activeWorld).toBe('saying')
    expect(store.activeEntryId).toBe(72)
  })

  it('registers a flush gate and uses the same save path for route updates', async () => {
    const gate = ref<(() => Promise<void>) | null>(null)
    const wrapper = await mountEditor(entry({ id: 73, revision: 5, content_md: '旧句' }), gate)

    await wrapper.get('textarea[aria-label="片语正文"]').setValue('新句子')
    await gate.value?.()
    await flushPromises()

    expect(api.updateEntry).toHaveBeenCalledWith(73, {
      revision: 5,
      content_md: '新句子',
      visibility: 'public',
      meta: { source: '摘录', author: '某人' },
    })

    await expect(navigation.updateGuard?.()).resolves.toBeUndefined()
    expect(api.updateEntry).toHaveBeenCalledTimes(2)
  })

  it('vetoes route changes when saving the current saying fails', async () => {
    api.updateEntry.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await mountEditor(entry({ id: 74, content_md: '原句' }))

    await wrapper.get('textarea[aria-label="片语正文"]').setValue('离线时修改')
    await expect(navigation.leaveGuard?.()).resolves.toBe(false)
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('稍后重试')
  })
})
