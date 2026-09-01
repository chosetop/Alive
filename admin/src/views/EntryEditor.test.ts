import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref, type Ref } from 'vue'

import WorkspaceHeader from '../components/writing/WorkspaceHeader.vue'
import ArticleSettings from '../components/writing/ArticleSettings.vue'
import { useWritingStore, writingFlushKey } from '../stores/writing'

import { ApiClientError, NETWORK_ERROR } from '../api/errors'
import type { EntryDetail, EntryListItem, EntryUpdateRequest } from '../types/api'
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
  nextPut: null as Promise<void> | null,
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
    worldsApi: { listAdminWorlds: vi.fn().mockResolvedValue([]) },
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
      const pending = recovery.nextPut
      recovery.nextPut = null
      if (pending !== null) await pending
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

function listItem(overrides: Partial<EntryListItem> = {}): EntryListItem {
  return {
    id: 1,
    world: 'journal',
    kind: '',
    type: 'journal',
    title: '文章',
    slug: 'article',
    summary: '',
    cover_url: '',
    meta: {},
    word_count: 0,
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

describe('EntryEditor autosave integration', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-26T12:00:00.000Z'))
    vi.resetAllMocks()
    recovery.records.clear()
    recovery.nextPut = null
    navigation.leaveGuard = null
    navigation.updateGuard = null
    api.listCategoriesAdmin.mockResolvedValue([
      {
        id: 7,
        world: 'journal',
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

    await (await titleField(wrapper)).setValue('新的标题')
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

    await (await titleField(wrapper)).setValue('保存中的标题')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('[data-save-status]').text()).toContain('保存中')

    save.resolve(entry({ ...current, revision: 2, title: '保存中的标题' }))
    await flushPromises()
    expect(wrapper.get('[data-save-status]').text()).toContain('已保存')
  })

  it('passes the current world label into the shared workspace header', async () => {
    const wrapper = await mountEditor(entry({ id: 46, world: 'video' }))

    expect(wrapper.get('[data-world-context]').text()).toContain('影像')
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
    await openSettings(wrapper)

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

    await wrapper.get('#e-category').setValue('7')
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenNthCalledWith(5, server.current.id, {
      revision: 5,
      category_id: 7,
    })

    await wrapper.get('input[type="radio"][value="private"]').setValue()
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenNthCalledWith(6, server.current.id, {
      revision: 6,
      visibility: 'private',
    })
  })

  it('prevents the browser save command and flushes immediately', async () => {
    const server = installMutableServer()
    const wrapper = await mountEditor(server.current)
    await openSettings(wrapper)
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

    await (await titleField(wrapper)).setValue('发布前标题')
    await wrapper.findAll('button').find((button) => button.text() === '发布')!.trigger('click')
    for (const checkbox of wrapper.findAll('input[type="checkbox"]')) await checkbox.setValue(true)
    await wrapper.get('[data-publish-confirm]').trigger('click')
    await flushPromises()

    expect(events).toEqual(['save:发布前标题', 'publish:2'])
    expect(api.publishEntry).toHaveBeenCalledWith(server.current.id, 2)

    await openSettings(wrapper)
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
    for (const checkbox of wrapper.findAll('input[type="checkbox"]')) await checkbox.setValue(true)
    await wrapper.get('[data-publish-confirm]').trigger('click')
    await flushPromises()

    expect(wrapper.get('textarea[aria-label="正文编辑器"]').attributes('disabled')).toBeDefined()
    publication.resolve(entry({ ...server.current, revision: 2, status: 'published' }))
    await flushPromises()
  })

  it('flushes pending edits before deleting and bypasses the leave flush after deletion', async () => {
    const events: string[] = []
    const server = installMutableServer(() => events.push('save'))
    api.deleteEntry.mockImplementation(async () => {
      events.push('delete')
    })
    navigation.replace.mockImplementationOnce(async () => events.push('navigate'))
    const wrapper = await mountEditor(server.current)
    await openSettings(wrapper)
    await wrapper.get('#e-summary').setValue('删除前保存')

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认删除')!.trigger('click')
    await flushPromises()

    expect(events).toEqual(['save', 'delete', 'navigate'])
    expect(api.updateEntry).toHaveBeenCalledOnce()
    expect(api.deleteEntry).toHaveBeenCalledWith(server.current.id)
    expect(navigation.replace).toHaveBeenCalledWith({ name: 'entry-blank' })
  })

  it('opens the next directory article after deleting the current article', async () => {
    const server = installMutableServer()
    navigation.replace.mockResolvedValue(undefined)
    const wrapper = await mountEditor(server.current)
    storeFor(wrapper).setDirectoryEntries([
      listItem({ id: server.current.id }),
      listItem({ id: 52 }),
      listItem({ id: 53 }),
    ])

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认删除')!.trigger('click')
    await flushPromises()

    expect(navigation.replace).toHaveBeenCalledWith({ name: 'entry-edit', params: { id: '52' } })
  })

  it('opens the previous directory article after deleting the last article', async () => {
    const server = installMutableServer()
    navigation.replace.mockResolvedValue(undefined)
    const wrapper = await mountEditor(server.current)
    storeFor(wrapper).setDirectoryEntries([listItem({ id: 51 }), listItem({ id: server.current.id })])

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认删除')!.trigger('click')
    await flushPromises()

    expect(navigation.replace).toHaveBeenCalledWith({ name: 'entry-edit', params: { id: '51' } })
  })

  it('keeps a blank writing canvas for an article absent from the directory', async () => {
    const server = installMutableServer()
    navigation.replace.mockResolvedValue(undefined)
    const wrapper = await mountEditor(server.current)
    storeFor(wrapper).setDirectoryEntries([listItem({ id: 51 })])

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认删除')!.trigger('click')
    await flushPromises()

    expect(navigation.replace).toHaveBeenCalledWith({ name: 'entry-blank' })
    expect(wrapper.find('[data-writing-blank]').exists()).toBe(true)
  })

  it('blocks edits while deletion is in flight and never patches the deleted entry', async () => {
    const server = installMutableServer()
    const deletion = deferred<void>()
    api.deleteEntry.mockReturnValue(deletion.promise)
    const wrapper = await mountEditor(server.current)

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认删除')!.trigger('click')
    await flushPromises()

    const title = await titleField(wrapper)
    expect(title.attributes('disabled')).toBeDefined()
    await title.setValue('删除等待期间输入')
    deletion.resolve(undefined)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)

    expect(api.updateEntry).not.toHaveBeenCalled()
    expect(navigation.replace).toHaveBeenCalledWith({ name: 'entry-blank' })
  })

  it('does not delete when the required pre-delete flush fails', async () => {
    const current = entry({ id: 48 })
    api.getEntry.mockResolvedValue(current)
    api.updateEntry.mockRejectedValue(
      new ApiClientError({ code: NETWORK_ERROR, status: 0, message: 'offline' }),
    )
    const wrapper = await mountEditor(current)
    await (await titleField(wrapper)).setValue('尚未保存')

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认删除')!.trigger('click')
    await flushPromises()

    expect(api.deleteEntry).not.toHaveBeenCalled()
    expect(navigation.replace).not.toHaveBeenCalled()
    expect(wrapper.get('[data-save-status]').text()).toContain('离线')
  })

  it('uses the same flush gate before leaving or switching entries', async () => {
    const server = installMutableServer()
    const wrapper = await mountEditor(server.current)
    await (await titleField(wrapper)).setValue('离开前保存')

    await expect(navigation.leaveGuard?.()).resolves.toBeUndefined()
    expect(api.updateEntry).toHaveBeenCalledOnce()

    await openSettings(wrapper)
    await wrapper.get('#e-summary').setValue('切换前保存')
    await expect(navigation.updateGuard?.()).resolves.toBeUndefined()
    expect(api.updateEntry).toHaveBeenCalledTimes(2)
  })

  it('ignores an older entry load that resolves after a faster route switch', async () => {
    const firstLoad = deferred<EntryDetail>()
    const newerEntry = entry({ id: 51, title: '新路由文章', revision: 7 })
    api.getEntry.mockImplementation((id: number) =>
      id === 50 ? firstLoad.promise : Promise.resolve(newerEntry),
    )
    api.updateEntry.mockResolvedValue(
      entry({ ...newerEntry, revision: 8, summary: '仍保存到新文章' }),
    )
    const wrapper = await mountEditorWithProps({ id: '50' })

    await wrapper.setProps({ id: '51' })
    await flushPromises()
    expect((await titleField(wrapper)).element).toHaveProperty('value', '新路由文章')

    firstLoad.resolve(entry({ id: 50, title: '过期慢响应' }))
    await flushPromises()

    expect((await titleField(wrapper)).element).toHaveProperty('value', '新路由文章')
    await openSettings(wrapper)
    await wrapper.get('#e-summary').setValue('仍保存到新文章')
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.updateEntry).toHaveBeenLastCalledWith(51, {
      revision: 7,
      summary: '仍保存到新文章',
    })
  })

  it('binds a preloaded existing entry without issuing a second getEntry call', async () => {
    const current = entry({ id: 67, world: 'video', revision: 4 })
    const wrapper = await mountEditorWithProps({ id: '67', initialEntry: current })

    expect(api.getEntry).not.toHaveBeenCalled()
    expect(storeFor(wrapper).activeWorld).toBe('video')
    expect((await titleField(wrapper)).element).toHaveProperty('value', '原始标题')
  })

  it('locks the previous entry while the next route loads and keeps it locked after failure', async () => {
    const previous = entry({ id: 57, revision: 4, title: '上一篇' })
    const nextLoad = deferred<EntryDetail>()
    api.getEntry.mockImplementation((id: number) =>
      id === previous.id ? Promise.resolve(previous) : nextLoad.promise,
    )
    api.updateEntry.mockResolvedValue(entry({ ...previous, revision: 5 }))
    const wrapper = await mountEditorWithProps({ id: String(previous.id) })
    const previousTitle = await titleField(wrapper)

    await wrapper.setProps({ id: '58' })
    await flushPromises()
    const showedLoading = wrapper.text().includes('载入中')
    const showedPreviousEditor = wrapper.find('#e-title').exists()

    await previousTitle.setValue('切换期间不应保存')
    nextLoad.reject(
      new ApiClientError({ code: NETWORK_ERROR, status: 0, message: 'next entry offline' }),
    )
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)

    expect.soft(showedLoading).toBe(true)
    expect.soft(showedPreviousEditor).toBe(false)
    expect.soft(wrapper.get('[role="alert"]').text()).toContain('无法连接到服务器')
    expect.soft(wrapper.find('#e-title').exists()).toBe(false)
    expect(api.updateEntry).not.toHaveBeenCalled()
  })

  it('restores the previous entry after the next route fails to load', async () => {
    const previous = entry({ id: 59, revision: 4, title: '可返回的上一篇' })
    const nextLoad = deferred<EntryDetail>()
    api.getEntry.mockImplementation((id: number) =>
      id === previous.id ? Promise.resolve(previous) : nextLoad.promise,
    )
    api.updateEntry.mockResolvedValue(entry({ ...previous, revision: 5, title: '返回后继续写' }))
    const wrapper = await mountEditorWithProps({ id: String(previous.id) })

    await wrapper.setProps({ id: '60' })
    await flushPromises()
    nextLoad.reject(
      new ApiClientError({ code: NETWORK_ERROR, status: 0, message: 'next entry offline' }),
    )
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('无法连接到服务器')

    await wrapper.setProps({ id: String(previous.id) })
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect((await titleField(wrapper)).element).toHaveProperty('value', '可返回的上一篇')
    expect((await titleField(wrapper)).attributes('disabled')).toBeUndefined()

    await (await titleField(wrapper)).setValue('返回后继续写')
    await vi.advanceTimersByTimeAsync(1000)

    expect(api.updateEntry).toHaveBeenCalledWith(previous.id, {
      revision: 4,
      title: '返回后继续写',
    })
  })

  it('does not navigate or bind autosave when draft creation resolves after unmount', async () => {
    const creation = deferred<EntryDetail>()
    api.createEntry.mockReturnValue(creation.promise)
    const wrapper = await mountEditorWithProps({})

    wrapper.unmount()
    creation.resolve(entry({ id: 88, revision: 1, title: '', slug: '' }))
    await flushPromises()

    expect(navigation.replace).not.toHaveBeenCalled()
    expect(api.updateEntry).not.toHaveBeenCalled()
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
    expect(actionLabels).toEqual(['载入服务端', '覆盖服务端', '另存为恢复草稿'])

    await editor.setValue('409 后继续写的正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', metaKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()
    expect(api.updateEntry).toHaveBeenCalledOnce()
    expect(recovery.records.get(46)).toMatchObject({
      fields: { content_md: '409 后继续写的正文' },
      syncState: 'conflict',
    })

    navigation.replace.mockImplementationOnce(async () => {
      const guardResult = await navigation.updateGuard?.()
      if (guardResult === false) throw new Error('navigation blocked by unresolved conflict')
    })

    await wrapper
      .findAll('[data-conflict-action]')
      .find((button) => button.text() === '另存为恢复草稿')!
      .trigger('click')
    await flushPromises()

    expect(api.createEntry).toHaveBeenCalledWith({ world: 'journal' })
    expect(api.updateEntry).toHaveBeenLastCalledWith(99, {
      revision: 1,
      content_md: '409 后继续写的正文',
    })
    expect(navigation.replace).toHaveBeenLastCalledWith({
      name: 'entry-edit',
      params: { id: '99' },
    })
    expect(wrapper.get('[data-save-status]').text()).toContain('已保存')
  })

  it('overwrites with recovered local fields at the current server revision', async () => {
    const current = entry({ id: 53, content_md: '旧服务端正文' })
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const latestServer = entry({ id: 53, revision: 8, content_md: '别人提交的正文' })
    const overwritten = entry({ id: 53, revision: 9, content_md: 'API 返回的正文' })
    api.updateEntry.mockRejectedValueOnce(conflict).mockResolvedValueOnce(overwritten)
    const wrapper = await mountEditor(current)
    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')

    await editor.setValue('要覆盖的本地正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', metaKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()
    api.getEntry.mockResolvedValue(latestServer)

    await wrapper
      .findAll('[data-conflict-action]')
      .find((button) => button.text() === '覆盖服务端')!
      .trigger('click')
    await flushPromises()

    expect(api.getEntry).toHaveBeenLastCalledWith(53)
    expect(api.updateEntry).toHaveBeenLastCalledWith(53, {
      revision: 8,
      content_md: '要覆盖的本地正文',
    })
    expect(recovery.records.has(53)).toBe(false)
    expect(wrapper.get('[data-save-status]').text()).toContain('已保存')
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toBe(editor.element)
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toHaveProperty(
      'value',
      '要覆盖的本地正文',
    )
  })

  it('waits for the latest conflict edit before overwriting the server', async () => {
    const current = entry({ id: 55, content_md: '旧服务端正文' })
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const latestServer = entry({ id: 55, revision: 6, content_md: '别人提交的正文' })
    const overwritten = entry({ id: 55, revision: 7, content_md: 'API 返回的正文' })
    const localWrite = deferred<void>()
    api.updateEntry.mockRejectedValueOnce(conflict).mockResolvedValueOnce(overwritten)
    const wrapper = await mountEditor(current)
    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')

    await editor.setValue('冲突发生时的正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', metaKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()
    recovery.nextPut = localWrite.promise
    await editor.setValue('点击覆盖前最后输入')
    api.getEntry.mockResolvedValue(latestServer)

    await wrapper
      .findAll('[data-conflict-action]')
      .find((button) => button.text() === '覆盖服务端')!
      .trigger('click')
    await flushPromises()
    const callsBeforeLocalWrite = api.updateEntry.mock.calls.length

    localWrite.resolve(undefined)
    await flushPromises()

    expect(callsBeforeLocalWrite).toBe(1)
    expect(api.updateEntry).toHaveBeenLastCalledWith(55, {
      revision: 6,
      content_md: '点击覆盖前最后输入',
    })
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toHaveProperty(
      'value',
      '点击覆盖前最后输入',
    )
  })

  it('keeps local content and recovery state when overwrite fails', async () => {
    const current = entry({ id: 54, content_md: '旧服务端正文' })
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const overwriteFailure = new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: 'connection lost',
    })
    api.updateEntry.mockRejectedValueOnce(conflict).mockRejectedValueOnce(overwriteFailure)
    const wrapper = await mountEditor(current)
    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')

    await editor.setValue('失败也要保留的正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()
    api.getEntry.mockResolvedValue(entry({ id: 54, revision: 6, content_md: '最新服务端正文' }))

    await wrapper
      .findAll('[data-conflict-action]')
      .find((button) => button.text() === '覆盖服务端')!
      .trigger('click')
    await flushPromises()

    expect(api.updateEntry).toHaveBeenLastCalledWith(54, {
      revision: 6,
      content_md: '失败也要保留的正文',
    })
    expect(recovery.records.get(54)).toMatchObject({
      fields: { content_md: '失败也要保留的正文' },
      syncState: 'conflict',
    })
    expect(wrapper.get('[data-save-status]').text()).toContain('保存冲突')
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toBe(editor.element)
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toHaveProperty(
      'value',
      '失败也要保留的正文',
    )
  })

  it('reuses the same recovery draft when its first patch attempt fails', async () => {
    const current = entry({ id: 52, content_md: '服务端正文' })
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const patchFailure = new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: 'connection lost',
    })
    const created = entry({ id: 120, revision: 4, title: '', slug: '', content_md: '' })
    const recovered = entry({ id: 120, revision: 5, content_md: '待恢复正文' })
    api.getEntry.mockResolvedValue(current)
    api.createEntry.mockResolvedValue(created)
    api.updateEntry
      .mockRejectedValueOnce(conflict)
      .mockRejectedValueOnce(patchFailure)
      .mockResolvedValueOnce(recovered)
    const wrapper = await mountEditor(current)

    await wrapper.get('textarea[aria-label="正文编辑器"]').setValue('待恢复正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', metaKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()
    const recoverButton = () =>
      wrapper
        .findAll('[data-conflict-action]')
        .find((button) => button.text() === '另存为恢复草稿')!

    await recoverButton().trigger('click')
    await flushPromises()
    expect(api.createEntry).toHaveBeenCalledOnce()
    expect(api.createEntry).toHaveBeenCalledWith({ world: 'journal' })
    expect(api.updateEntry).toHaveBeenNthCalledWith(2, 120, {
      revision: 4,
      content_md: '待恢复正文',
    })
    expect(navigation.replace).not.toHaveBeenCalled()

    await recoverButton().trigger('click')
    await flushPromises()

    expect(api.createEntry).toHaveBeenCalledOnce()
    expect(api.createEntry).toHaveBeenCalledWith({ world: 'journal' })
    expect(api.updateEntry).toHaveBeenNthCalledWith(3, 120, {
      revision: 4,
      content_md: '待恢复正文',
    })
    expect(navigation.replace).toHaveBeenCalledWith({
      name: 'entry-edit',
      params: { id: '120' },
    })
  })

  it('waits for the latest conflict edit before creating a recovery draft', async () => {
    const current = entry({ id: 56, content_md: '服务端正文' })
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const created = entry({ id: 130, revision: 4, title: '', slug: '', content_md: '' })
    const recovered = entry({ id: 130, revision: 5, content_md: 'API 返回的陈旧正文' })
    const localWrite = deferred<void>()
    api.updateEntry.mockRejectedValueOnce(conflict).mockResolvedValueOnce(recovered)
    api.createEntry.mockResolvedValue(created)
    const wrapper = await mountEditor(current)
    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')

    await editor.setValue('冲突发生时的正文')
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true }),
    )
    await flushPromises()
    recovery.nextPut = localWrite.promise
    await editor.setValue('点击另存前最后输入')

    await wrapper
      .findAll('[data-conflict-action]')
      .find((button) => button.text() === '另存为恢复草稿')!
      .trigger('click')
    await flushPromises()
    const createsBeforeLocalWrite = api.createEntry.mock.calls.length

    localWrite.resolve(undefined)
    await flushPromises()

    expect(createsBeforeLocalWrite).toBe(0)
    expect(api.updateEntry).toHaveBeenLastCalledWith(130, {
      revision: 4,
      content_md: '点击另存前最后输入',
    })
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').element).toHaveProperty(
      'value',
      '点击另存前最后输入',
    )
  })

  it('does not create a server draft until the new-entry route is edited', async () => {
    const created = entry({ id: 99, revision: 1, title: '', slug: '', content_md: '' })
    api.createEntry.mockResolvedValue(created)
    api.updateEntry.mockImplementation(async (_id: number, body: EntryUpdateRequest) =>
      entry({ ...created, ...body, revision: body.revision + 1 }),
    )
    const wrapper = await mountEditor(undefined)

    expect(api.createEntry).not.toHaveBeenCalled()
    expect(navigation.replace).not.toHaveBeenCalled()
    expect(wrapper.findAll('button').some((button) => button.text() === '保存')).toBe(false)

    await wrapper.get('textarea[aria-label="正文编辑器"]').setValue('新草稿正文')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.createEntry).toHaveBeenCalledWith({ world: 'journal' })
    expect(api.updateEntry).toHaveBeenCalledWith(99, { revision: 1, content_md: '新草稿正文' })
  })

  it('keeps a created draft usable when loading categories fails', async () => {
    const created = entry({ id: 140, revision: 3, title: '', slug: '', content_md: '' })
    api.createEntry.mockResolvedValue(created)
    api.listCategoriesAdmin.mockRejectedValue(
      new ApiClientError({ code: NETWORK_ERROR, status: 0, message: 'taxonomy offline' }),
    )
    api.updateEntry.mockImplementation(async (_id: number, body: EntryUpdateRequest) =>
      entry({ ...created, ...body, revision: body.revision + 1 }),
    )

    const wrapper = await mountEditor(undefined)

    expect(api.createEntry).not.toHaveBeenCalled()
    expect(api.createEntry).not.toHaveBeenCalled()

    await wrapper.get('textarea[aria-label="正文编辑器"]').setValue('分类失败仍可编辑')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.createEntry).toHaveBeenCalledWith({ world: 'journal' })

    expect(api.updateEntry).toHaveBeenCalledWith(140, {
      revision: 3,
      content_md: '分类失败仍可编辑',
    })
  })

  it('renders offline and ordinary errors in the fixed save-status slot', async () => {
    const current = entry({ id: 47 })
    api.getEntry.mockResolvedValue(current)
    api.updateEntry.mockRejectedValueOnce(
      new ApiClientError({ code: NETWORK_ERROR, status: 0, message: 'offline' }),
    )
    const wrapper = await mountEditor(current)

    await (await titleField(wrapper)).setValue('离线标题')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('[data-save-status]').text()).toContain('离线')

    api.updateEntry.mockRejectedValueOnce(
      new ApiClientError({ code: 'INVALID_INPUT', status: 400, message: 'bad title' }),
    )
    await wrapper.get('[data-save-retry]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-save-status]').text()).toContain('保存失败')
  })

  /**
   * The editor's half of the writing shell contract. The directory cannot flush
   * for itself and does not know which article is open, so both facts have to
   * arrive from here -- and a break in either is silent: switching articles would
   * drop the last keystroke, or the directory would highlight nothing.
   */
  describe('writing shell integration', () => {
    it('keeps metadata controls in the settings panel instead of the writing canvas', async () => {
      const wrapper = await mountEditor(entry({ id: 60 }))

      expect(wrapper.getComponent({ name: 'ArticleSettings' })).toBeTruthy()
      expect(wrapper.find('#e-slug').exists()).toBe(false)
      expect(wrapper.find('#e-type').exists()).toBe(false)
      expect(wrapper.find('#e-category').exists()).toBe(false)
      expect(wrapper.find('#e-summary').exists()).toBe(false)
      expect(wrapper.find('#e-cover').exists()).toBe(false)
      expect(wrapper.find('#e-happened').exists()).toBe(false)
      expect(wrapper.find('fieldset.fieldset').exists()).toBe(false)
    })

    it('publishes the open article id for the directory to highlight', async () => {
      const current = entry({ id: 61 })
      const wrapper = await mountEditor(current)

      expect(storeFor(wrapper).activeEntryId).toBe(61)
    })

    it('does not publish an id for an untouched new-article route', async () => {
      api.createEntry.mockResolvedValue(entry({ id: 62, revision: 1, title: '' }))
      const wrapper = await mountEditorWithProps({})

      expect(storeFor(wrapper).activeEntryId).toBeNull()
    })

    it('clears the open article id on unmount', async () => {
      const wrapper = await mountEditor(entry({ id: 63 }))
      const store = storeFor(wrapper)
      expect(store.activeEntryId).toBe(63)

      wrapper.unmount()
      activeWrappers.splice(activeWrappers.indexOf(wrapper), 1)
      await flushPromises()

      // Otherwise a row stays highlighted over a canvas showing something else.
      expect(store.activeEntryId).toBeNull()
    })

    it('registers a flush gate that drains pending fields', async () => {
      const gate = ref<(() => Promise<void>) | null>(null)
      const server = installMutableServer()
      const wrapper = await mountEditorWithProps({ id: String(server.current.id) }, gate)

      expect(gate.value).not.toBeNull()

      await (await titleField(wrapper)).setValue('目录切换前的标题')
      // Called before the debounce elapses, which is the case that matters: the
      // directory clicks while the timer still holds the last keystroke.
      await gate.value?.()
      await flushPromises()

      expect(api.updateEntry).toHaveBeenCalledWith(server.current.id, {
        revision: 1,
        title: '目录切换前的标题',
      })
    })

    it('leaves a successor editor gate in place when it unmounts', async () => {
      const gate = ref<(() => Promise<void>) | null>(null)
      api.getEntry.mockImplementation(async (id: number) => entry({ id }))
      const outgoing = await mountEditorWithProps({ id: '64' }, gate)
      const first = gate.value

      const incoming = await mountEditorWithProps({ id: '65' }, gate)
      expect(gate.value).not.toBe(first)
      const second = gate.value

      // Vue mounts the incoming editor before unmounting the outgoing one, so an
      // unconditional clear here would erase a live gate and make the next article
      // switch skip its flush entirely.
      outgoing.unmount()
      activeWrappers.splice(activeWrappers.indexOf(outgoing), 1)
      await flushPromises()

      expect(gate.value).toBe(second)
      incoming.unmount()
      activeWrappers.splice(activeWrappers.indexOf(incoming), 1)
    })

    it('routes the header menu into the status transitions', async () => {
      const current = entry({ id: 66, status: 'published' })
      api.getEntry.mockResolvedValue(current)
      api.unpublishEntry.mockResolvedValue(entry({ ...current, revision: 2, status: 'draft' }))
      const wrapper = await mountEditor(current)

      // Dispatched by id from the header component, not by clicking through the
      // portal: the wiring under test is the id-to-endpoint mapping.
      wrapper.getComponent(WorkspaceHeader).vm.$emit('action', 'unpublish')
      await flushPromises()

      expect(api.unpublishEntry).toHaveBeenCalledWith(66, 1)
    })
  })
})

async function mountEditor(current: EntryDetail | undefined): Promise<VueWrapper> {
  if (current !== undefined) api.getEntry.mockResolvedValue(current)
  return mountEditorWithProps(current === undefined ? {} : { id: String(current.id) })
}

function storeFor(wrapper: VueWrapper) {
  return useWritingStore(wrapper.vm.$.appContext.config.globalProperties.$pinia)
}

async function mountEditorWithProps(
  props: { id?: string; initialEntry?: EntryDetail },
  flushGate?: Ref<(() => Promise<void>) | null>,
): Promise<VueWrapper> {
  const wrapper = mount(EntryEditor, {
    props,
    global: {
      provide: flushGate === undefined ? {} : { [writingFlushKey as symbol]: flushGate },
      // A fresh Pinia per mount. The editor publishes the open article's id into
      // the writing store for the directory to highlight, and a shared instance
      // would leak that id between tests -- which would make the unmount-clears-it
      // behaviour untestable.
      plugins: [createPinia()],
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
  activeWrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

async function openSettings(wrapper: VueWrapper): Promise<void> {
  wrapper.getComponent(WorkspaceHeader).vm.$emit('action', 'settings')
  await flushPromises()
  expect(wrapper.getComponent(ArticleSettings).props('open')).toBe(true)
}

async function titleField(wrapper: VueWrapper) {
  if (!wrapper.find('#e-title').exists()) await openSettings(wrapper)
  return wrapper.get('#e-title')
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
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-26T00:00:00Z',
    ...overrides,
  }
}

function deferred<T>(): {
  promise: Promise<T>
  resolve(value: T): void
  reject(error: unknown): void
} {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}
