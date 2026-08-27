import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'

import type { SaveStatus } from '../../editor/save-coordinator'
import { useWritingStore } from '../../stores/writing'
import type { EntryStatus } from '../../types/api'
import WorkspaceHeader from './WorkspaceHeader.vue'

/**
 * The header's job is to say what the save state is without moving anything, and
 * to keep the destructive actions away from the primary one. Both are properties
 * an ordinary-looking edit can break: a status string that only differs by colour,
 * or an action promoted out of the menu onto the bar.
 */

function mountHeader(
  props: { saveStatus?: SaveStatus; entryStatus?: EntryStatus | null; busy?: boolean } = {},
): VueWrapper {
  return mount(WorkspaceHeader, {
    props: { saveStatus: 'saved', ...props },
    global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
  })
}

describe('WorkspaceHeader', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('gives the mobile conflict actions their own full-width row', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'WorkspaceHeader.vue'), 'utf8')

    expect(source).toMatch(/grid-template-areas:\s*['"]left right['"]\s*['"]status status['"]/
    )
    expect(source).toMatch(/\.status\s*\{[\s\S]*overflow:\s*visible/)
    expect(source).toMatch(/\.status-slot\s*\{[\s\S]*flex-wrap:\s*wrap/)
  })

  it('describes every save state in words', () => {
    // Not colour, not an animated dot. A writer who cannot tell "已保存" from
    // "保存失败" has no way to know whether to worry, and the greyscale case is
    // the one that exposes a status carried by hue alone.
    const expected: Array<[SaveStatus, string]> = [
      ['saved', '已保存'],
      ['pending', '待保存'],
      ['saving', '保存中'],
      ['offline', '离线'],
      ['error', '保存失败'],
      ['conflict', '保存冲突'],
    ]

    for (const [status, text] of expected) {
      const wrapper = mountHeader({ saveStatus: status })
      expect(wrapper.get('[data-save-status]').text(), status).toContain(text)
      wrapper.unmount()
    }
  })

  it('keeps the status region mounted in every state', () => {
    // A region that appears only once there is something to say is not observed
    // by a screen reader in time to announce it.
    for (const status of ['saved', 'pending', 'saving', 'offline', 'error', 'conflict'] as const) {
      const wrapper = mountHeader({ saveStatus: status })
      const region = wrapper.get('[data-save-status]')
      expect(region.attributes('aria-live'), status).toBe('polite')
      wrapper.unmount()
    }
  })

  it('offers retry only for the two states a retry can fix', () => {
    for (const status of ['offline', 'error'] as const) {
      const wrapper = mountHeader({ saveStatus: status })
      expect(wrapper.find('[data-save-retry]').exists(), status).toBe(true)
      wrapper.unmount()
    }

    // Not during `saving` -- a second flush would race the first -- and not during
    // `conflict`, where the choice is which version survives, not whether to try
    // again. That choice belongs to the editor, which is what the slot is for.
    for (const status of ['saved', 'pending', 'saving', 'conflict'] as const) {
      const wrapper = mountHeader({ saveStatus: status })
      expect(wrapper.find('[data-save-retry]').exists(), status).toBe(false)
      wrapper.unmount()
    }
  })

  it('emits retry rather than saving anything itself', async () => {
    const wrapper = mountHeader({ saveStatus: 'error' })

    await wrapper.get('[data-save-retry]').trigger('click')

    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('renders the conflict slot only while the save is conflicted', () => {
    const conflicted = mount(WorkspaceHeader, {
      props: { saveStatus: 'conflict' },
      slots: { conflict: '<button data-test="choice">载入服务端</button>' },
    })
    expect(conflicted.find('[data-test="choice"]').exists()).toBe(true)

    const saved = mount(WorkspaceHeader, {
      props: { saveStatus: 'saved' },
      slots: { conflict: '<button data-test="choice">载入服务端</button>' },
    })
    expect(saved.find('[data-test="choice"]').exists()).toBe(false)
  })

  it('carries no save button at all', () => {
    // The spec removes it: everything is autosaved, and a visible save control
    // teaches that not pressing it loses work.
    const wrapper = mountHeader({ saveStatus: 'pending', entryStatus: 'draft' })

    expect(wrapper.findAll('button').some((button) => button.text() === '保存')).toBe(false)
  })

  it('shows the directory toggle only while the directory is hidden', async () => {
    const wrapper = mountHeader()
    const store = useWritingStore()

    // Open by default, and the collapse control lives inside the directory itself,
    // so the header would otherwise show a second control for the same thing.
    expect(wrapper.find('[data-directory-expand]').exists()).toBe(false)

    store.setDirectoryOpen(false)
    await nextTick()

    const toggle = wrapper.get('[data-directory-expand]')
    // Named, because it is icon-only: an unnamed one is announced as "button".
    expect(toggle.attributes('aria-label')).toBe('展开文章目录')

    await toggle.trigger('click')
    expect(store.directoryOpen).toBe(true)
  })

  it('offers no publish or menu before the record exists', () => {
    // Publishing something never created has no meaning, and neither does
    // archiving it.
    const wrapper = mountHeader({ entryStatus: null })

    expect(wrapper.find('[data-publish]').exists()).toBe(false)
    expect(wrapper.find('[data-more-actions]').exists()).toBe(false)
    // Preview stays: it describes the canvas, which exists either way.
    expect(wrapper.find('[data-preview]').exists()).toBe(true)
  })

  it('places deletion in the header with explicit confirmation', () => {
    const wrapper = mountHeader({ entryStatus: 'draft' })

    expect(wrapper.get('[data-header-delete]').text()).toBe('删除')
    expect(wrapper.find('[data-header-delete-confirm]').exists()).toBe(false)
    expect(wrapper.get('[data-publish]').text()).toBe('发布')
  })

  it('does not offer publish for something already published', () => {
    const wrapper = mountHeader({ entryStatus: 'published' })

    // Reads as state rather than as an action, and cannot be pressed: publishing
    // twice is a 400 the writer has no way to interpret.
    expect(wrapper.get('[data-publish]').text()).toBe('已发布')
    expect(wrapper.get('[data-publish]').attributes('disabled')).toBeDefined()
  })

  it('disables publish while the editor is busy', () => {
    const wrapper = mountHeader({ entryStatus: 'draft', busy: true })

    expect(wrapper.get('[data-publish]').attributes('disabled')).toBeDefined()
  })

  it('emits publish and preview without acting on them', async () => {
    const wrapper = mountHeader({ entryStatus: 'draft' })

    await wrapper.get('[data-publish]').trigger('click')
    await wrapper.get('[data-preview]').trigger('click')

    expect(wrapper.emitted('publish')).toHaveLength(1)
    expect(wrapper.emitted('preview')).toHaveLength(1)
  })

  /**
   * Menu items live in a portal at the body, so these need a real document to
   * attach to and two macrotask turns to settle Reka's open sequence -- the same
   * reasoning as the primitive layer's own tests.
   */
  async function openMenu(entryStatus: EntryStatus): Promise<{
    wrapper: VueWrapper
    labels: string[]
  }> {
    const wrapper = mount(WorkspaceHeader, {
      props: { saveStatus: 'saved', entryStatus },
      attachTo: document.body,
    })
    await wrapper.get('[data-more-actions]').trigger('click')
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 0))
    await new Promise((resolve) => setTimeout(resolve, 0))
    await nextTick()

    const labels = [...document.querySelectorAll('[role="menuitem"]')].map(
      (node) => node.textContent?.trim() ?? '',
    )
    return { wrapper, labels }
  }

  it('offers unpublish only for a published article', async () => {
    const published = await openMenu('published')
    expect(published.labels).toContain('撤回为草稿')
    published.wrapper.unmount()

    // A draft has nothing to withdraw, and the endpoint would 400.
    const draft = await openMenu('draft')
    expect(draft.labels).not.toContain('撤回为草稿')
    draft.wrapper.unmount()
  })

  it('offers archive only for something not already archived', async () => {
    const draft = await openMenu('draft')
    expect(draft.labels).toContain('归档')
    draft.wrapper.unmount()

    const archived = await openMenu('archived')
    expect(archived.labels).not.toContain('归档')
    archived.wrapper.unmount()
  })

  it('reports the chosen action by id', async () => {
    const { wrapper } = await openMenu('draft')
    const archive = [...document.querySelectorAll('[role="menuitem"]')].find(
      (node) => node.textContent?.trim() === '归档',
    )

    ;(archive as HTMLElement).click()
    await nextTick()

    // The id, never the index: the item list is computed from the entry's status,
    // so an index would point at a different action for a published article.
    expect(wrapper.emitted('action')?.[0]).toEqual(['archive'])
    wrapper.unmount()
  })

  /**
   * jsdom does no layout, so the no-jitter guarantee cannot be observed by
   * measuring anything. These read the stylesheet instead. Weaker than a rendered
   * measurement, and still the only thing that would object if the pinned height
   * were dropped -- which is a one-line edit whose symptom (the canvas twitching
   * every time a save resolves) nobody would attribute to this file.
   */
  const source = readFileSync(
    resolve(dirname(fileURLToPath(import.meta.url)), 'WorkspaceHeader.vue'),
    'utf8',
  )

  it('pins the status slot to a fixed height', () => {
    const statusRule = source.match(/\.status \{[^}]*\}/)?.[0] ?? ''

    expect(statusRule).toMatch(/height:\s*[\d.]+rem/)
    // Clipped rather than wrapped: a second line is the same jitter by another
    // route, and 离线，本地草稿已保留 is long enough to wrap in a narrow header.
    expect(statusRule).toContain('white-space: nowrap')
  })

  it('styles itself from semantic tokens only', () => {
    // A literal colour here would be the one element that ignores the theme once
    // Plan 3 lands, and in a dark theme it reads as a rendering fault.
    const styles = source.slice(source.indexOf('<style'))

    expect(styles).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
    expect(styles).not.toMatch(/\brgba?\(/)
    expect(styles).not.toMatch(/\bhsla?\(/)
  })
})
