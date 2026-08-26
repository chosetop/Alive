import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiClientError, NETWORK_ERROR } from '../api/errors'
import type { EntryDetail, EntryUpdateRequest } from '../types/api'
import EntryEditor from './EntryEditor.vue'

const api = vi.hoisted(() => ({
  listCategoriesAdmin: vi.fn(),
  getEntry: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  publishEntry: vi.fn(),
  unpublishEntry: vi.fn(),
  archiveEntry: vi.fn(),
  deleteEntry: vi.fn(),
}))

const navigation = vi.hoisted(() => ({
  replace: vi.fn(),
  leaveGuard: null as null | (() => Promise<boolean | void>),
  updateGuard: null as null | (() => Promise<boolean | void>),
}))

const recovery = vi.hoisted(() => ({
  records: new Map<number, unknown>(),
}))

vi.mock('../api', async () => {
  const patch = await import('../api/patch')
  const errors = await import('../api/errors')
  return {
    ...patch,
    ...errors,
    categoriesApi: { listCategoriesAdmin: api.listCategoriesAdmin },
    entriesApi: {
      getEntry: api.getEntry,
      createEntry: api.createEntry,
      updateEntry: api.updateEntry,
      publishEntry: api.publishEntry,
      unpublishEntry: api.unpublishEntry,
      archiveEntry: api.archiveEntry,
      deleteEntry: api.deleteEntry,
    },
  }
})

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRouter: () => ({ replace: navigation.replace }),
    onBeforeRouteLeave: (guard: () => Promise<boolean | void>) => {
      navigation.leaveGuard = guard
    },
    onBeforeRouteUpdate: (guard: () => Promise<boolean | void>) => {
      navigation.updateGuard = guard
    },
  }
})

vi.mock('../editor/recovery-store', () => ({
  EntryRecoveryStore: class {
    async get(entryId: number) {
      return recovery.records.get(entryId) ?? null
    }

    async put(record: { entryId: number }) {
      recovery.records.set(record.entryId, structuredClone(record))
    }

    async remove(entryId: number) {
      recovery.records.delete(entryId)
    }
  },
}))

vi.mock('../components/MarkdownEditor.vue', async () => {
  const { defineComponent, ref } = await import('vue')
  return {
    default: defineComponent({
      name: 'MarkdownEditor',
      props: {
        initialValue: { type: String, required: true },
        disabled: { type: Boolean, default: false },
      },
      emits: ['update'],
      setup(props) {
        return { localValue: ref(props.initialValue) }
      },
      template:
        '<textarea aria-label="正文编辑器" v-model="localValue" :disabled="disabled" @input="$emit(\'update\', localValue)" />',
    }),
  }
})

const activeWrappers: VueWrapper[] = []

describe('EntryEditor autosave integration', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-26T12:00:00.000Z'))
    vi.clearAllMocks()
    recovery.records.clear()
    navigation.leaveGuard = null
    navigation.updateGuard = null
    api.listCategoriesAdmin.mockResolvedValue([
      {
        id: 7,
        name: '随笔',
        slug: 'notes',
        description: '',
        sort_order: 0,
        created_at: '2026-08-01T00:00:00Z',
        updated_at: '2026-08-01T00:00:00Z',
      },
    ])
    api.createEntry.mockResolvedValue(entry({ id: 99, revision: 1, title: '', slug: '' }))
    api.deleteEntry.mockResolvedValue(undefined)
  })

  afterEach(async () => {
    for (const wrapper of activeWrappers.splice(0)) wrapper.unmount()
    await flushPromises()
    vi.useRealTimers()
  })

  it('debounces a title edit into a revision-aware field-only update', async () => {
    const server = installMutableServer()
    const wrapper = await mountEditor(server.current)

    await wrapper.get('#e-title').setValue('新的标题')
    expect(wrapper.get('[data-save-status]').text()).toContain('待保存')
    await vi.advanceTimersByTimeAsync(999)
    expect(api.updateEntry).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)

    expect(api.updateEntry).toHaveBeenCalledWith(server.current.id, {
      revision: 1,
      title: '新的标题',
    })
    expect(wrapper.get('[data-save-status]').text()).toContain('已保存')
  })

  it('renders the saving state in the same status slot while the request is in flight', async () => {
    const current = entry({ id: 45 })
    const save = deferred<EntryDetail>()
    api.getEntry.mockResolvedValue(current)
    api.updateEntry.mockReturnValue(save.promise)
    const wrapper = await mountEditor(current)

    await wrapper.get('#e-title').setValue('保存中的标题')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('[data-save-status]').text()).toContain('保存中')

    save.resolve(entry({ ...current, revision: 2, title: '保存中的标题' }))
    await flushPromises()
    expect(wrapper.get('[data-save-status]').text()).toContain('已保存')
  })

  it('schedules Milkdown markdown as content_md without remounting the editor', async () => {
    const server = installMutableServer()
    api.updateEntry.mockResolvedValueOnce(
      entry({ ...server.current, revision: 2, content_md: '服务端返回的不同正文' }),
    )
    const wrapper = await mountEditor(server.current)
    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')

    await editor.setValue('本地正在写的正文')
    await vi.advanceTimersByTimeAsync(1000)

    expect(api.updateEntry).toHaveBeenCalledWith(server.current.id, {
      revision: 1,
      content_md: '本地正在写的正文',
    })
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toBe(editor.element)
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toHaveProperty(
      'value',
      '本地正在写的正文',
    )
  })

  it('queues every remaining form field with only its API field', async () => {
    const server = installMutableServer()
    const wrapper = await mountEditor(server.current)

    const changes: Array<[string, string, unknown]> = [
      ['#e-slug', 'new-slug', 'new-slug'],
      ['#e-summary', '新摘要', '新摘要'],
      ['#e-cover', 'https://example.com/cover.jpg', 'https://example.com/cover.jpg'],
      ['#e-happened', '2026-08-27T09:30', '2026-08-27T01:30:00.000Z'],
    ]
    const apiFields = ['slug', 'summary', 'cover_url', 'happened_at']

    for (const [index, [selector, formValue, apiValue]] of changes.entries()) {
      await wrapper.get(selector).setValue(formValue)
      await vi.advanceTimersByTimeAsync(1000)
      expect(api.updateEntry).toHaveBeenNthCalledWith(index + 1, server.current.id, {
        revision: index + 1,
        [apiFields[index] as string]: apiValue,
      })
    }

    await wrapper.get('#e-type').setValue('book')
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenNthCalledWith(5, server.current.id, {
      revision: 5,
      type: 'book',
    })

    await wrapper.get('#e-category').setValue('7')
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenNthCalledWith(6, server.current.id, {
      revision: 6,
      category_id: 7,
    })

    await wrapper.get('input[type="radio"][value="private"]').setValue()
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenNthCalledWith(7, server.current.id, {
      revision: 7,
      visibility: 'private',
    })
  })

  it('prevents the browser save command and flushes immediately', async () => {
    const server = installMutableServer()
    const wrapper = await mountEditor(server.current)
    await wrapper.get('#e-summary').setValue('快捷保存')
    const plainSave = new KeyboardEvent('keydown', {
      key: 's',
      bubbles: true,
      cancelable: true,
    })
    const otherCommand = new KeyboardEvent('keydown', {
      key: 'p',
      metaKey: true,
      bubbles: true,
      cancelable: true,
    })
    const event = new KeyboardEvent('keydown', {
      key: 's',
      metaKey: true,
      bubbles: true,
      cancelable: true,
    })

    window.dispatchEvent(plainSave)
    window.dispatchEvent(otherCommand)
    window.dispatchEvent(event)
    await flushPromises()

    expect(plainSave.defaultPrevented).toBe(false)
    expect(otherCommand.defaultPrevented).toBe(false)
    expect(event.defaultPrevented).toBe(true)
    expect(api.updateEntry).toHaveBeenCalledWith(server.current.id, {
      revision: 1,
      summary: '快捷保存',
    })
  })

  it('flushes pending fields before publishing and transitions at the saved revision', async () => {
    const events: string[] = []
    const server = installMutableServer((body) => {
      events.push(`save:${String(body.title)}`)
    })
    api.publishEntry.mockImplementation(async (_id: number, revision: number) => {
      events.push(`publish:${revision}`)
      return entry({ ...server.current, revision: revision + 1, status: 'published' })
    })
    const wrapper = await mountEditor(server.current)

    await wrapper.get('#e-title').setValue('发布前标题')
    await wrapper.findAll('button').find((button) => button.text() === '发布')!.trigger('click')
    await flushPromises()

    expect(events).toEqual(['save:发布前标题', 'publish:2'])
    expect(api.publishEntry).toHaveBeenCalledWith(server.current.id, 2)

    await wrapper.get('#e-summary').setValue('发布后的编辑')
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenLastCalledWith(server.current.id, {
      revision: 3,
      summary: '发布后的编辑',
    })
  })

  it('prevents Milkdown edits while a status transition is in flight', async () => {
    const server = installMutableServer()
    const publication = deferred<EntryDetail>()
    api.publishEntry.mockReturnValue(publication.promise)
    const wrapper = await mountEditor(server.current)

    await wrapper.findAll('button').find((button) => button.text() === '发布')!.trigger('click')
    await flushPromises()

    expect(wrapper.get('textarea[aria-label="正文编辑器"]').attributes('disabled')).toBeDefined()
    publication.resolve(entry({ ...server.current, revision: 2, status: 'published' }))
    await flushPromises()
  })

  it('uses the same flush gate before leaving or switching entries', async () => {
    const server = installMutableServer()
    const wrapper = await mountEditor(server.current)
    await wrapper.get('#e-title').setValue('离开前保存')

    await expect(navigation.leaveGuard?.()).resolves.toBeUndefined()
    expect(api.updateEntry).toHaveBeenCalledOnce()

    await wrapper.get('#e-summary').setValue('切换前保存')
    await expect(navigation.updateGuard?.()).resolves.toBeUndefined()
    expect(api.updateEntry).toHaveBeenCalledTimes(2)
  })

  it('keeps local markdown on a 409 and can create a recovery draft from IndexedDB', async () => {
    const current = entry({ id: 46, content_md: '服务端正文' })
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const recovered = entry({ id: 99, revision: 2, content_md: '本地冲突正文' })
    api.getEntry.mockResolvedValue(current)
    api.updateEntry.mockRejectedValueOnce(conflict).mockResolvedValueOnce(recovered)
    api.createEntry.mockResolvedValue(entry({ id: 99, revision: 1, title: '', slug: '' }))
    const wrapper = await mountEditor(current)

    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')
    await editor.setValue('本地冲突正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()

    expect(wrapper.get('[data-save-status]').text()).toContain('保存冲突')
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toBe(editor.element)
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toHaveProperty(
      'value',
      '本地冲突正文',
    )
    const actionLabels = wrapper.findAll('[data-conflict-action]').map((button) => button.text())
    expect(actionLabels).toEqual(['载入服务端', '另存为恢复草稿'])

    navigation.replace.mockImplementationOnce(async () => {
      const guardResult = await navigation.updateGuard?.()
      if (guardResult === false) throw new Error('navigation blocked by unresolved conflict')
    })

    await wrapper
      .findAll('[data-conflict-action]')
      .find((button) => button.text() === '另存为恢复草稿')!
      .trigger('click')
    await flushPromises()

    expect(api.createEntry).toHaveBeenCalledWith({})
    expect(api.updateEntry).toHaveBeenLastCalledWith(99, {
      revision: 1,
      content_md: '本地冲突正文',
    })
    expect(navigation.replace).toHaveBeenLastCalledWith({
      name: 'entry-edit',
      params: { id: '99' },
    })
    expect(wrapper.get('[data-save-status]').text()).toContain('已保存')
  })

  it('creates an empty draft before binding autosave on the new-entry route', async () => {
    const created = entry({ id: 99, revision: 1, title: '', slug: '', content_md: '' })
    api.createEntry.mockResolvedValue(created)
    api.updateEntry.mockImplementation(async (_id: number, body: EntryUpdateRequest) =>
      entry({ ...created, ...body, revision: body.revision + 1 }),
    )
    const wrapper = await mountEditor(undefined)

    expect(api.createEntry).toHaveBeenCalledWith({})
    expect(navigation.replace).toHaveBeenCalledWith({
      name: 'entry-edit',
      params: { id: '99' },
    })
    expect(wrapper.findAll('button').some((button) => button.text() === '保存')).toBe(false)

    await wrapper.get('#e-title').setValue('新草稿标题')
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenCalledWith(99, { revision: 1, title: '新草稿标题' })
  })

  it('renders offline and ordinary errors in the fixed save-status slot', async () => {
    const current = entry({ id: 47 })
    api.getEntry.mockResolvedValue(current)
    api.updateEntry.mockRejectedValueOnce(
      new ApiClientError({ code: NETWORK_ERROR, status: 0, message: 'offline' }),
    )
    const wrapper = await mountEditor(current)

    await wrapper.get('#e-title').setValue('离线标题')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('[data-save-status]').text()).toContain('离线')

    api.updateEntry.mockRejectedValueOnce(
      new ApiClientError({ code: 'INVALID_INPUT', status: 400, message: 'bad title' }),
    )
    await wrapper.get('[data-save-retry]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-save-status]').text()).toContain('保存失败')
  })
})

async function mountEditor(current: EntryDetail | undefined): Promise<VueWrapper> {
  if (current !== undefined) api.getEntry.mockResolvedValue(current)
  const wrapper = mount(EntryEditor, {
    props: current === undefined ? {} : { id: String(current.id) },
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
  activeWrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

function installMutableServer(onSave?: (body: EntryUpdateRequest) => void): {
  current: EntryDetail
} {
  const server = { current: entry() }
  api.getEntry.mockImplementation(async () => server.current)
  api.updateEntry.mockImplementation(async (_id: number, body: EntryUpdateRequest) => {
    onSave?.(body)
    server.current = entry({
      ...server.current,
      ...body,
      revision: body.revision + 1,
      content_md: body.content_md ?? server.current.content_md,
      cover_url: body.cover_url ?? server.current.cover_url,
      category_id: body.category_id ?? server.current.category_id,
      happened_at: body.happened_at ?? server.current.happened_at,
    })
    return server.current
  })
  return server
}

function entry(overrides: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 42,
    revision: 1,
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
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-26T00:00:00Z',
    ...overrides,
  }
}

function deferred<T>(): { promise: Promise<T>; resolve(value: T): void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
