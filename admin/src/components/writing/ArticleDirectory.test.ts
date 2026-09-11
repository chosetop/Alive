import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import * as VueRouter from 'vue-router'

import { useWritingStore, writingFlushKey, type WritingFlushGate } from '../../stores/writing'
import type { EntryListItem } from '../../types/api'
import ArticleDirectory from './ArticleDirectory.vue'

/**
 * These cover what the directory decides, not what it renders. The list requests
 * it issues, when it issues them, and what it does to the open article before
 * navigating are the parts a later refactor can silently change while the pane
 * still looks right.
 */

const api = vi.hoisted(() => ({
  listEntriesAdmin: vi.fn(),
  listCategoriesAdmin: vi.fn(),
  createEntry: vi.fn(),
  deleteEntry: vi.fn(),
}))

const recovery = vi.hoisted(() => ({
  records: [] as Array<{ entryId: number }>,
  listRejection: null as Error | null,
}))

const navigation = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('../../api', async () => {
  const errors = await import('../../api/errors')
  return {
    ...errors,
    entriesApi: { listEntriesAdmin: api.listEntriesAdmin, createEntry: api.createEntry, deleteEntry: api.deleteEntry },
    categoriesApi: { listCategoriesAdmin: api.listCategoriesAdmin },
  }
})

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return { ...actual, useRouter: () => ({ push: navigation.push }) }
})

vi.mock('../../editor/recovery-store', () => ({
  EntryRecoveryStore: class {
    async list() {
      if (recovery.listRejection !== null) throw recovery.listRejection
      return recovery.records
    }
  },
}))

const wrappers: VueWrapper[] = []

function item(overrides: Partial<EntryListItem> = {}): EntryListItem {
  return {
    id: 1,
    world: 'journal',
    kind: '',
    type: 'journal',
    title: '山中一日',
    slug: 'a-day',
    summary: '',
    cover_url: '',
    meta: {},
    word_count: 100,
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

function page(data: EntryListItem[]) {
  return { data, meta: { page: 1, page_size: 20, total: data.length } }
}

async function mountDirectory(
  options: { drawer?: boolean; flush?: WritingFlushGate; world?: 'journal' | 'saying' | 'video' } = {},
): Promise<VueWrapper> {
  const wrapper = mount(ArticleDirectory, {
    props: { drawer: options.drawer ?? false, world: options.world ?? 'journal' },
    global: {
      provide: options.flush === undefined ? {} : { [writingFlushKey as symbol]: options.flush },
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('ArticleDirectory', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.resetAllMocks()
    recovery.records = []
    recovery.listRejection = null
    setActivePinia(createPinia())
    api.listCategoriesAdmin.mockResolvedValue([])
    api.listEntriesAdmin.mockResolvedValue(page([item()]))
    api.deleteEntry.mockResolvedValue(undefined)
  })

  afterEach(async () => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
    await flushPromises()
    vi.useRealTimers()
  })

  it('loads recent articles with the active world and a page size of 20', async () => {
    await mountDirectory({ world: 'saying' })

    // No status: "recent" spans every status, ordered by last edit. Passing one
    // would quietly turn the top of the pane into a draft list.
    expect(api.listEntriesAdmin).toHaveBeenCalledExactlyOnceWith({ world: 'saying', page_size: 20 })
    expect(useWritingStore().directoryEntries.map((entry) => entry.id)).toEqual([1])
  })

  it('makes the sidebar Alive wordmark return to Dashboard', async () => {
    const wrapper = await mountDirectory()

    const brand = wrapper.get('[data-directory-brand]')
    expect(brand.text()).toBe('Alive')
    expect(brand.attributes('to')).toBe('/dashboard')
    expect(brand.attributes('aria-label')).toBe('返回 Dashboard')
  })

  it('groups the current world by category and removes the article library', async () => {
    api.listCategoriesAdmin.mockResolvedValue([
      { id: 3, world: 'journal', name: '旅行', slug: 'travel', description: '', created_at: '', updated_at: '' },
    ])
    api.listEntriesAdmin.mockResolvedValue(page([
      item({ id: 7, category: { id: 3, name: '旅行', slug: 'travel' } }),
      item({ id: 8, category: null }),
    ]))

    const wrapper = await mountDirectory()

    expect(wrapper.get('[data-directory-category="travel"]').text()).toContain('山中一日')
    expect(wrapper.get('[data-directory-category="__uncategorised"]').text()).toContain('未分类')
    expect(wrapper.text()).not.toContain('文章库')
    expect(wrapper.text()).not.toContain('最近')
  })

  it('lets each category reveal or hide its own articles', async () => {
    api.listCategoriesAdmin.mockResolvedValue([
      { id: 3, world: 'journal', name: '旅行', slug: 'travel', description: '', created_at: '', updated_at: '' },
    ])
    api.listEntriesAdmin.mockResolvedValue(page([
      item({ id: 7, title: '山中一日', category: { id: 3, name: '旅行', slug: 'travel' } }),
    ]))
    const wrapper = await mountDirectory()
    const toggle = wrapper.get('[data-category-toggle="travel"]')

    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.text()).toContain('山中一日')

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.text()).not.toContain('山中一日')

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.text()).toContain('山中一日')
  })

  it('falls back to recent only when the current world has no categories', async () => {
    const wrapper = await mountDirectory()

    expect(wrapper.get('#directory-recent-heading').text()).toBe('最近')
    expect(wrapper.find('[data-directory-category]').exists()).toBe(false)
  })

  it('debounces the search by 250ms into a single request', async () => {
    const wrapper = await mountDirectory({ world: 'saying' })
    const store = useWritingStore()
    const search = wrapper.get('[data-directory-search]')

    await search.setValue('山')
    await search.setValue('山中')
    await vi.advanceTimersByTimeAsync(249)
    // Still nothing at 249ms: every keystroke firing would queue the directory's
    // own requests behind the autosave PATCHes it must not delay.
    expect(api.listEntriesAdmin).toHaveBeenCalledOnce()

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()

    expect(api.listEntriesAdmin).toHaveBeenLastCalledWith({ world: 'saying', q: '山中', page_size: 20 })
    expect(api.listEntriesAdmin).toHaveBeenCalledTimes(2)
    expect(store.searchQuery).toBe('山中')
  })

  it('uses the registered world copy in labels and create navigation', async () => {
    const order: string[] = []
    const flush: WritingFlushGate = ref(async () => {
      order.push('flush')
    })
    navigation.push.mockImplementation(async () => {
      order.push('navigate')
    })
    const wrapper = await mountDirectory({ world: 'saying', flush })
    useWritingStore().setActiveEntry(1)

    expect(wrapper.get('nav[aria-label="片语目录"]').element).toBeTruthy()
    expect(wrapper.get('[data-directory-new]').text()).toContain('写片语')
    expect(wrapper.get('[data-directory-search]').attributes('placeholder')).toBe('搜索片语')

    await wrapper.get('[data-directory-new]').trigger('click')
    await flushPromises()

    expect(api.createEntry).not.toHaveBeenCalled()
    expect(order).toEqual(['flush', 'navigate'])
    expect(navigation.push).toHaveBeenCalledWith(expect.objectContaining({ name: 'saying-editor-new', params: { world: 'saying' }, query: expect.any(Object) }))
  })

  it('treats a whitespace-only query as no query at all', async () => {
    const wrapper = await mountDirectory()

    await wrapper.get('[data-directory-search]').setValue('   ')
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    // The backend drops a blank `q`, so searching one would return the whole
    // directory under a "搜索结果" heading that misdescribes it.
    expect(api.listEntriesAdmin).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-directory-recent]').exists()).toBe(true)
  })

  it('keeps the newest results when an earlier search resolves last', async () => {
    // Two searches in flight, older one answering second. Clearing the box is not
    // enough to test this: an empty query hides the results section outright, so
    // that assertion holds with the guard deleted. Overtaking responses are the
    // case only the generation counter can catch, and the wrong outcome is a pane
    // showing matches for a query that is no longer in the box.
    const wrapper = await mountDirectory()
    let resolveFirst: (value: unknown) => void = () => {}
    api.listEntriesAdmin.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveFirst = resolve
      }),
    )
    const search = wrapper.get('[data-directory-search]')

    await search.setValue('山')
    await vi.advanceTimersByTimeAsync(250)

    api.listEntriesAdmin.mockResolvedValueOnce(page([item({ id: 3, title: '第二次搜索的结果' })]))
    await search.setValue('海')
    await vi.advanceTimersByTimeAsync(250)
    await flushPromises()
    expect(wrapper.text()).toContain('第二次搜索的结果')

    resolveFirst(page([item({ id: 2, title: '第一次搜索的结果' })]))
    await flushPromises()

    expect(wrapper.text()).toContain('第二次搜索的结果')
    expect(wrapper.text()).not.toContain('第一次搜索的结果')
  })

  it('marks articles that have unsaved local edits', async () => {
    // The dot comes from the recovery store, not the list response: the server
    // cannot know about an edit that never reached it, which is the whole case.
    recovery.records = [{ entryId: 2 }]
    api.listEntriesAdmin.mockResolvedValue(
      page([item({ id: 1 }), item({ id: 2, title: '有未同步改动' })]),
    )
    const wrapper = await mountDirectory()

    const rows = wrapper.findAll('[data-directory-recent] [data-entry-id]')
    expect(rows[0]?.find('[data-unsynced]').exists()).toBe(false)
    expect(rows[1]?.find('[data-unsynced]').exists()).toBe(true)
    // Text, not only a coloured dot. The spec requires state to be perceivable
    // without relying on colour.
    expect(rows[1]?.get('[data-unsynced]').text()).toContain('未同步')
  })

  it('falls back to 无标题草稿 without writing that title anywhere', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 3, title: '   ' })]))
    const wrapper = await mountDirectory()

    expect(wrapper.get('[data-entry-id="3"]').text()).toContain('无标题草稿')
    // Shown, never saved: a fallback that reached the record would become a real
    // title somebody has to delete.
    expect(api.createEntry).not.toHaveBeenCalled()
  })

  it('flushes the open article before navigating to another one', async () => {
    const order: string[] = []
    const flush: WritingFlushGate = ref(async () => {
      order.push('flush')
    })
    navigation.push.mockImplementation(async () => {
      order.push('navigate')
    })
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 4 }), item({ id: 5 })]))
    const wrapper = await mountDirectory({ flush })
    useWritingStore().setActiveEntry(4)

    await wrapper.get('[data-entry-id="5"]').trigger('click')
    await flushPromises()

    // Ordering, not just occurrence: navigating first would leave the last
    // keystroke sitting in a debounce timer belonging to an article that is no
    // longer on screen.
    expect(order).toEqual(['flush', 'navigate'])
    expect(navigation.push).toHaveBeenCalledWith({ name: 'entry-edit', params: { id: '5' } })
  })

  it('reports a navigation veto instead of silently doing nothing', async () => {
    navigation.push.mockRejectedValue(new Error('navigation aborted by guard'))
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 5 })]))
    const wrapper = await mountDirectory()

    await wrapper.get('[data-entry-id="5"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('无法切换文章')
  })

  it('reports a resolved navigation failure instead of closing the drawer', async () => {
    navigation.push.mockResolvedValue({ type: 4 })
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 5 })]))
    const wrapper = await mountDirectory({ drawer: true })

    await wrapper.get('[data-entry-id="5"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('无法切换文章')
    expect(useWritingStore().directoryOpen).toBe(true)
  })

  it('reports a flush veto instead of silently doing nothing', async () => {
    const flush: WritingFlushGate = ref(async () => {
      throw new Error('flush vetoed')
    })
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 6 })]))
    const wrapper = await mountDirectory({ flush })

    await wrapper.get('[data-entry-id="6"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('无法切换文章')
    expect(navigation.push).not.toHaveBeenCalled()
  })

  it('navigates without a flush when no editor is mounted', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 6 })]))
    // No gate provided at all: the drawer can be open over a canvas with no
    // article loaded, and awaiting a gate nobody registered would hang the tap.
    const wrapper = await mountDirectory()

    await wrapper.get('[data-entry-id="6"]').trigger('click')
    await flushPromises()

    expect(navigation.push).toHaveBeenCalledWith({ name: 'entry-edit', params: { id: '6' } })
  })

  it('does not navigate when the current article is clicked again', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 7 })]))
    const wrapper = await mountDirectory()
    useWritingStore().setActiveEntry(7)

    await wrapper.get('[data-entry-id="7"]').trigger('click')
    await flushPromises()

    expect(navigation.push).not.toHaveBeenCalled()
  })

  it('enters recent management mode and reveals selection controls', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 7 }), item({ id: 8 })]))
    const wrapper = await mountDirectory()

    expect(wrapper.find('[data-recent-select]').exists()).toBe(false)
    await wrapper.get('[data-recent-manage]').trigger('click')

    expect(wrapper.findAll('[data-recent-select]')).toHaveLength(2)
    expect(wrapper.find('[data-recent-delete]').exists()).toBe(false)
    await wrapper.get('[data-recent-select="7"]').setValue(true)
    expect(wrapper.get('[data-recent-delete]').text()).toBe('删除')
  })

  it('deletes all selected recent entries after one confirmation', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 7 }), item({ id: 8 }), item({ id: 9 })]))
    const wrapper = await mountDirectory()

    await wrapper.get('[data-recent-manage]').trigger('click')
    await wrapper.get('[data-recent-select="7"]').setValue(true)
    await wrapper.get('[data-recent-select="9"]').setValue(true)
    await wrapper.get('[data-recent-delete]').trigger('click')

    expect(api.deleteEntry).not.toHaveBeenCalled()
    expect(wrapper.get('[data-recent-delete-confirm]').text()).toContain('2')

    await wrapper.get('[data-recent-delete-confirm]').trigger('click')
    await flushPromises()

    expect(api.deleteEntry).toHaveBeenNthCalledWith(1, 7)
    expect(api.deleteEntry).toHaveBeenNthCalledWith(2, 9)
    expect(wrapper.find('[data-entry-id="7"]').exists()).toBe(false)
    expect(wrapper.find('[data-entry-id="8"]').exists()).toBe(true)
    expect(wrapper.find('[data-entry-id="9"]').exists()).toBe(false)
  })

  it('marks the open article as current for assistive technology', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 8 }), item({ id: 9 })]))
    const wrapper = await mountDirectory()
    useWritingStore().setActiveEntry(9)
    await flushPromises()

    expect(wrapper.get('[data-entry-id="8"]').attributes('aria-current')).toBeUndefined()
    expect(wrapper.get('[data-entry-id="9"]').attributes('aria-current')).toBe('true')
  })

  it('opens the client-first new route for the current world after flushing', async () => {
    const order: string[] = []
    const flush: WritingFlushGate = ref(async () => {
      order.push('flush')
    })
    navigation.push.mockImplementation(async () => {
      order.push('navigate')
    })
    const wrapper = await mountDirectory({ flush })
    useWritingStore().setActiveEntry(1)

    await wrapper.get('[data-directory-new]').trigger('click')
    await flushPromises()

    expect(api.createEntry).not.toHaveBeenCalled()
    expect(order).toEqual(['flush', 'navigate'])
    expect(navigation.push).toHaveBeenCalledWith({ name: 'entry-new-world', params: { world: 'journal' } })
  })

  it('disables the new button while the new-route navigation is in flight', async () => {
    let resolveNavigate: (value: unknown) => void = () => {}
    navigation.push.mockReturnValue(
      new Promise((resolve) => {
        resolveNavigate = resolve
      }),
    )
    const wrapper = await mountDirectory()

    await wrapper.get('[data-directory-new]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-directory-new]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-directory-new]').attributes('aria-busy')).toBe('true')

    resolveNavigate(undefined)
    await flushPromises()

    expect(navigation.push).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-directory-new]').attributes('disabled')).toBeUndefined()
  })

  it('reports a failed new-route navigation instead of silently doing nothing', async () => {
    navigation.push.mockRejectedValue(new Error('boom'))
    const wrapper = await mountDirectory()

    await wrapper.get('[data-directory-new]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').element).toBeTruthy()
  })

  it('closes the drawer after a successful new-route navigation', async () => {
    navigation.push.mockResolvedValue(undefined)
    const store = useWritingStore()
    const wrapper = await mountDirectory({ drawer: true })

    await wrapper.get('[data-directory-new]').trigger('click')
    await flushPromises()

    expect(navigation.push).toHaveBeenCalledWith({ name: 'entry-new-world', params: { world: 'journal' } })
    expect(store.directoryOpen).toBe(false)
  })

  it('keeps the drawer open when the new-route navigation is vetoed', async () => {
    vi.spyOn(VueRouter, 'isNavigationFailure').mockReturnValue(true)
    navigation.push.mockResolvedValue({ type: 4 })
    const store = useWritingStore()
    const wrapper = await mountDirectory({ drawer: true })

    await wrapper.get('[data-directory-new]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('当前编辑状态未改变')
    expect(store.directoryOpen).toBe(true)
  })

  it('closes the drawer after a selection, and only in drawer mode', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 11 })]))
    const store = useWritingStore()

    const column = await mountDirectory({ drawer: false })
    await column.get('[data-entry-id="11"]').trigger('click')
    await flushPromises()
    // The desktop column stays: it is the persistent pane, not an overlay.
    expect(store.directoryOpen).toBe(true)

    const drawer = await mountDirectory({ drawer: true })
    await drawer.get('[data-entry-id="11"]').trigger('click')
    await flushPromises()
    // The drawer covers the canvas, so leaving it open would hide the article it
    // was just used to open.
    expect(store.directoryOpen).toBe(false)
  })

  it('closes the drawer when the current article is tapped, having nothing to navigate to', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 12 })]))
    const store = useWritingStore()
    const wrapper = await mountDirectory({ drawer: true })
    store.setActiveEntry(12)

    await wrapper.get('[data-entry-id="12"]').trigger('click')
    await flushPromises()

    expect(navigation.push).not.toHaveBeenCalled()
    expect(store.directoryOpen).toBe(false)
  })

  it('collapses the directory from its own control', async () => {
    const wrapper = await mountDirectory()
    const store = useWritingStore()

    await wrapper.get('[data-directory-collapse]').trigger('click')

    expect(store.directoryOpen).toBe(false)
  })

  it('names the collapse control differently in a drawer', async () => {
    // "收起" describes hiding a column; the same word on a modal is wrong, and an
    // icon-only control has nothing else to go on.
    const column = await mountDirectory({ drawer: false })
    expect(column.get('[data-directory-collapse]').attributes('aria-label')).toBe('收起日志目录')
    expect(column.get('[data-directory-collapse] .ui-icon').attributes('aria-hidden')).toBe('true')

    const drawer = await mountDirectory({ drawer: true })
    expect(drawer.get('[data-directory-collapse]').attributes('aria-label')).toBe('关闭日志目录')
  })

  it('still renders the article list when the recovery store throws', async () => {
    // The pane is how you reach your work. Losing the dots is acceptable; losing
    // the list because IndexedDB is blocked is not. `list` has to actually reject
    // here -- an empty record array would pass with the catch removed.
    recovery.listRejection = new Error('IndexedDB blocked')
    const wrapper = await mountDirectory()

    expect(wrapper.get('[data-directory-recent]').element).toBeTruthy()
    expect(wrapper.findAll('[data-unsynced]')).toHaveLength(0)
  })

  it('surfaces a failed recent load as an alert rather than an empty pane', async () => {
    api.listEntriesAdmin.mockRejectedValue(new Error('boom'))
    const wrapper = await mountDirectory()

    expect(wrapper.get('[role="alert"]').element).toBeTruthy()
  })

  it('is announced as a world-scoped navigation landmark', async () => {
    const wrapper = await mountDirectory({ world: 'video' })

    // nav, not a bare div: this is how a screen reader user reaches the pane
    // without walking the canvas.
    expect(wrapper.get('nav[aria-label="影像目录"]').element).toBeTruthy()
  })

  it('identifies its translucent writing-desk surface and current-article cursor', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ id: 8 }), item({ id: 9 })]))
    const wrapper = await mountDirectory()
    useWritingStore().setActiveEntry(9)
    await flushPromises()

    expect(wrapper.get('[data-article-directory]').attributes('data-surface')).toBe('glass')
    expect(wrapper.get('[data-entry-id="9"] [data-alive-cursor]').attributes('aria-hidden')).toBe(
      'true',
    )
  })

  it('shows saying body excerpts with a fade container instead of title text', async () => {
    api.listEntriesAdmin.mockResolvedValue(page([item({ world: 'saying', title: '标题不应展示', content_md: '这是片语正文内容' } as never)]))
    const wrapper = await mountDirectory({ world: 'saying' })

    expect(wrapper.get('[data-saying-excerpt]').text()).toBe('这是片语正文内容')
    expect(wrapper.find('[data-saying-excerpt]').classes()).toContain('item-title--excerpt')
    expect(wrapper.text()).not.toContain('标题不应展示')
  })

  it('clamps saying excerpts to two lines while fading only from the right', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'ArticleDirectory.vue'), 'utf8')
    const excerptStyle = source.match(/\.item-title--excerpt\s*\{([\s\S]*?)\}/)?.[1] ?? ''
    const fadeStyle = source.match(/\.item-title--excerpt::after\s*\{([\s\S]*?)\}/)?.[1] ?? ''

    expect(excerptStyle).toMatch(/-webkit-line-clamp:\s*2/)
    expect(excerptStyle).toMatch(/line-clamp:\s*2/)
    expect(fadeStyle).toMatch(/right:\s*0/)
    expect(fadeStyle).toMatch(/background:\s*linear-gradient\(90deg/)
    expect(fadeStyle).not.toMatch(/linear-gradient\(180deg/)
  })

  it('keeps selected metadata aligned with the indented title', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'ArticleDirectory.vue'), 'utf8')
    expect(source).toMatch(/\.item\[aria-current='true'\]\s+\.item-meta\s*\{[\s\S]*?padding-inline-start:\s*var\(--space-2\)/)
  })
})
