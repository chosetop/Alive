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

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve
    reject = nextReject
  })
  return { promise, resolve, reject }
}

async function mountEditor(
  current?: EntryDetail,
  flushGate?: Ref<(() => Promise<void>) | null>,
): Promise<VueWrapper> {
  const wrapper = mount(SayingEditor, {
    props: current === undefined ? {} : { initialEntry: current },
    global: {
      provide: flushGate === undefined ? {} : { [writingFlushKey as symbol]: flushGate },
      plugins: [createPinia()],
      stubs: {
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      },
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

  it('renders the shared workspace header with the saying world label', async () => {
    const wrapper = await mountEditor(entry({ id: 78 }))

    expect(wrapper.find('[data-workspace-header]').exists()).toBe(true)
    expect(wrapper.get('[data-world-context]').text()).toContain('片语')
    expect(wrapper.findAll('[data-save-status]')).toHaveLength(1)
  })

  it('passes the current entry status into the shared header without exposing long-form actions', async () => {
    const wrapper = await mountEditor(entry({ id: 79, status: 'published' }))

    expect(wrapper.find('.entry-status').text()).toBe('已发布')
    expect(wrapper.find('[data-publish]').exists()).toBe(true)
    expect(wrapper.find('[data-header-settings]').exists()).toBe(false)
    expect(wrapper.find('[data-header-delete]').exists()).toBe(false)
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

  it('retries a failed draft creation through the shared header action', async () => {
    api.createEntry
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(entry({ id: 91, revision: 1, content_md: '' }))
    const wrapper = await mountEditor()

    await wrapper.get('textarea[aria-label="片语正文"]').setValue('重试后的句子')
    await flushPromises()

    expect(wrapper.get('[data-save-retry]').exists()).toBe(true)

    await wrapper.get('[data-save-retry]').trigger('click')
    await flushPromises()
    await flushPromises()

    expect(api.createEntry).toHaveBeenCalledTimes(2)
    expect(api.updateEntry).toHaveBeenCalledWith(91, {
      revision: 1,
      content_md: '重试后的句子',
      visibility: 'public',
      meta: { source: '', author: '' },
    })
  })

  it('waits for an in-flight save before allowing route leave', async () => {
    const gate = ref<(() => Promise<void>) | null>(null)
    const pendingSave = deferred<EntryDetail>()
    api.updateEntry.mockReturnValueOnce(pendingSave.promise)
    const wrapper = await mountEditor(entry({ id: 75, revision: 6, content_md: '旧句' }), gate)

    await wrapper.get('textarea[aria-label="片语正文"]').setValue('保存中的句子')
    const savePromise = gate.value?.()
    await Promise.resolve()

    const leavePromise = navigation.leaveGuard?.()
    let settled = false
    void leavePromise?.then(() => {
      settled = true
    })
    await Promise.resolve()

    expect(api.updateEntry).toHaveBeenCalledTimes(1)
    expect(settled).toBe(false)

    pendingSave.resolve(entry({ id: 75, revision: 7, content_md: '保存中的句子' }))
    await expect(savePromise).resolves.toBeUndefined()
    await expect(leavePromise).resolves.toBeUndefined()
    expect(api.updateEntry).toHaveBeenCalledTimes(1)
  })

  it('waits for an in-flight save failure before vetoing route leave', async () => {
    const gate = ref<(() => Promise<void>) | null>(null)
    const pendingSave = deferred<EntryDetail>()
    api.updateEntry.mockReturnValueOnce(pendingSave.promise)
    const wrapper = await mountEditor(entry({ id: 76, revision: 7, content_md: '旧句' }), gate)

    await wrapper.get('textarea[aria-label="片语正文"]').setValue('失败中的句子')
    const savePromise = gate.value?.()
    await Promise.resolve()

    const leavePromise = navigation.leaveGuard?.()
    let settled = false
    void leavePromise?.then(() => {
      settled = true
    })
    await Promise.resolve()

    expect(api.updateEntry).toHaveBeenCalledTimes(1)
    expect(settled).toBe(false)

    pendingSave.reject(new Error('offline'))
    await expect(savePromise).resolves.toBeUndefined()
    await expect(leavePromise).resolves.toBe(false)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('稍后重试')
  })

  it('flushes edits made while an earlier save is in flight', async () => {
    const gate = ref<(() => Promise<void>) | null>(null)
    const first = deferred<EntryDetail>()
    api.updateEntry.mockReturnValueOnce(first.promise).mockImplementationOnce(async (_id: number, body: Record<string, unknown>) =>
      entry({ revision: 9, content_md: String(body.content_md ?? '') }),
    )
    const wrapper = await mountEditor(entry({ id: 77, revision: 8, content_md: '旧句' }), gate)

    await wrapper.get('textarea[aria-label="片语正文"]').setValue('第一版')
    const firstSave = gate.value?.()
    await Promise.resolve()
    await wrapper.get('textarea[aria-label="片语正文"]').setValue('最终版')
    const leavePromise = navigation.leaveGuard?.()

    first.resolve(entry({ id: 77, revision: 9, content_md: '第一版' }))
    await firstSave
    await leavePromise
    await flushPromises()
    await flushPromises()

    expect(api.updateEntry).toHaveBeenNthCalledWith(2, 77, expect.objectContaining({ revision: 9, content_md: '最终版' }))
  })
})
