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
    global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } },
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

  it('marks every save state with the Alive cursor without replacing its text', () => {
    for (const status of ['saved', 'pending', 'saving', 'offline', 'error', 'conflict'] as const) {
      const wrapper = mountHeader({ saveStatus: status })
      const cursor = wrapper.get('[data-save-cursor]')

      expect(cursor.attributes('data-status'), status).toBe(status)
      expect(cursor.attributes('aria-hidden'), status).toBe('true')
      expect(wrapper.get('[data-save-status]').text(), status).not.toBe('')
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
    expect(toggle.get('.ui-icon').attributes('aria-hidden')).toBe('true')

    await toggle.trigger('click')
    expect(store.directoryOpen).toBe(true)
  })

  it('links the writing brand back to Dashboard', () => {
    const wrapper = mountHeader()
    const brand = wrapper.get('[data-writing-brand]')

    expect(brand.attributes('href')).toBe('/dashboard')
    expect(brand.text()).toBe('Alive')
  })

  it('offers no publish or menu before the record exists', () => {
    // Publishing something never created has no meaning, and neither does
    // archiving it.
    const wrapper = mountHeader({ entryStatus: null })

    expect(wrapper.find('[data-publish]').exists()).toBe(false)
    expect(wrapper.find('[data-more-actions]').exists()).toBe(false)
    expect(wrapper.find('[data-preview]').exists()).toBe(false)
  })

  it('keeps writing actions directly visible instead of hiding them behind more', () => {
    const wrapper = mountHeader({ entryStatus: 'draft' })

    expect(wrapper.find('[data-more-actions]').exists()).toBe(false)
    expect(wrapper.get('[data-header-settings]').text()).toContain('设置')
    expect(wrapper.get('[data-header-archive]').text()).toContain('归档')
  })

  it('places deletion in the header with explicit confirmation', () => {
    const wrapper = mountHeader({ entryStatus: 'draft' })

    expect(wrapper.get('[data-header-delete]').text()).toBe('删除')
    expect(wrapper.get('[data-more-actions] .ui-icon').attributes('aria-hidden')).toBe('true')
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

  it('emits publish without exposing a preview action', async () => {
    const wrapper = mountHeader({ entryStatus: 'draft' })

    await wrapper.get('[data-publish]').trigger('click')

    expect(wrapper.emitted('publish')).toHaveLength(1)
    expect(wrapper.find('[data-preview]').exists()).toBe(false)
  })

  it('offers unpublish only for a published article', async () => {
    const published = mountHeader({ entryStatus: 'published' })
    expect(published.get('[data-header-unpublish]').text()).toContain('撤回')

    const draft = mountHeader({ entryStatus: 'draft' })
    expect(draft.find('[data-header-unpublish]').exists()).toBe(false)
  })

  it('offers archive only for something not already archived', async () => {
    expect(mountHeader({ entryStatus: 'draft' }).get('[data-header-archive]').text()).toContain('归档')
    expect(mountHeader({ entryStatus: 'archived' }).find('[data-header-archive]').exists()).toBe(false)
  })

  it('reports direct action buttons by id', async () => {
    const wrapper = mountHeader({ entryStatus: 'draft' })
    await wrapper.get('[data-header-settings]').trigger('click')
    await wrapper.get('[data-header-archive]').trigger('click')

    expect(wrapper.emitted('action')).toEqual([['settings'], ['archive']])
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
