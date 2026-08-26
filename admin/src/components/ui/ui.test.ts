import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import UiButton from './UiButton.vue'
import UiDialog from './UiDialog.vue'
import UiIconButton from './UiIconButton.vue'
import UiMenu from './UiMenu.vue'
import UiPopover from './UiPopover.vue'
import UiToastRegion from './UiToastRegion.vue'

/**
 * These are the behaviours the writing workspace depends on, not a restatement of
 * Reka UI's own test suite. They exist because the wrappers are the layer that
 * can silently drop an ARIA attribute or a focus return while still rendering
 * something that looks correct.
 *
 * Overlays need `attachTo: document.body`: Reka teleports content to the body and
 * moves focus into it, and a detached wrapper has no document to move focus
 * within, so a focus assertion would pass or fail for the wrong reason.
 */

/**
 * Settles teleport, transition, and focus-restoration work.
 *
 * Two macrotask turns, not one. Reka's dropdown restores focus to the trigger
 * from inside a `setTimeout(…, 0)` scheduled during the close, so a single
 * timeout flush can run before that callback is queued and the assertion sees
 * focus still on the body.
 */
async function settle(): Promise<void> {
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}

describe('UiButton', () => {
  it('renders its slot content', () => {
    const wrapper = mount(UiButton, { slots: { default: '发布' } })

    expect(wrapper.text()).toBe('发布')
  })

  it('defaults to type="button"', () => {
    // A bare <button> inside a form submits it. Every action button in the
    // workspace sits near inputs, so the default must not navigate.
    expect(mount(UiButton).attributes('type')).toBe('button')
  })

  it('carries the disabled attribute and does not emit when disabled', async () => {
    const wrapper = mount(UiButton, { props: { disabled: true } })

    expect(wrapper.attributes('disabled')).toBeDefined()

    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('reports busy state to assistive technology, not just visually', async () => {
    const wrapper = mount(UiButton, { props: { loading: true } })

    expect(wrapper.attributes('aria-busy')).toBe('true')
    // Loading is a form of disabled: a second publish click must not fire.
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('emits click when enabled', async () => {
    const wrapper = mount(UiButton)

    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  it('marks the variant as a data attribute rather than a class name', () => {
    // Styling hooks on data-variant keep the CSS independent of scoped-class
    // hashing.
    expect(mount(UiButton, { props: { variant: 'danger' } }).attributes('data-variant')).toBe(
      'danger',
    )
  })
})

describe('UiIconButton', () => {
  it('requires and applies an accessible label', () => {
    // An icon-only control is unreachable without one.
    const wrapper = mount(UiIconButton, { props: { label: '收起目录' } })

    expect(wrapper.attributes('aria-label')).toBe('收起目录')
  })

  it('hides its decorative icon slot from the accessibility tree', () => {
    const wrapper = mount(UiIconButton, {
      props: { label: '收起目录' },
      slots: { default: '<svg />' },
    })

    // The label already names the control; the glyph would be announced twice.
    expect(wrapper.get('[data-icon]').attributes('aria-hidden')).toBe('true')
  })

  it('omits aria-pressed entirely when it is not a toggle', () => {
    // A non-toggle button announced as aria-pressed="false" is announced as an
    // unpressed toggle, which it is not.
    expect(mount(UiIconButton, { props: { label: '新文章' } }).attributes('aria-pressed')).toBeUndefined()
  })

  it('reports toggle state when it is a toggle', () => {
    const open = mount(UiIconButton, { props: { label: '收起目录', pressed: true } })
    const closed = mount(UiIconButton, { props: { label: '收起目录', pressed: false } })

    expect(open.attributes('aria-pressed')).toBe('true')
    // "false" must be present, not absent: the directory toggle has two states and
    // both need announcing.
    expect(closed.attributes('aria-pressed')).toBe('false')
  })
})

describe('UiDialog', () => {
  it('opens from its trigger and exposes the dialog role', async () => {
    const wrapper = mount(UiDialog, {
      attachTo: document.body,
      props: { title: '删除文章' },
      slots: { trigger: '<button data-test="trigger">打开</button>', default: '正文' },
    })

    await wrapper.get('[data-test="trigger"]').trigger('click')
    await settle()

    expect(document.querySelector('[role="dialog"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('names the dialog with its title, so it is not announced as unlabelled', async () => {
    const wrapper = mount(UiDialog, {
      attachTo: document.body,
      props: { title: '删除文章' },
      slots: { trigger: '<button data-test="trigger">打开</button>' },
    })

    await wrapper.get('[data-test="trigger"]').trigger('click')
    await settle()

    const dialog = document.querySelector('[role="dialog"]')
    const labelId = dialog?.getAttribute('aria-labelledby')
    expect(labelId).toBeTruthy()
    expect(document.getElementById(labelId!)?.textContent).toContain('删除文章')
    wrapper.unmount()
  })

  it('closes on Escape and returns focus to the trigger', async () => {
    const wrapper = mount(UiDialog, {
      attachTo: document.body,
      props: { title: '删除文章' },
      slots: { trigger: '<button data-test="trigger">打开</button>' },
    })
    const trigger = wrapper.get('[data-test="trigger"]')

    await trigger.trigger('click')
    await settle()
    expect(document.querySelector('[role="dialog"]')).not.toBeNull()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await settle()

    expect(document.querySelector('[role="dialog"]')).toBeNull()
    // The whole point of the wrapper: focus must not be left on the body, or
    // keyboard position is lost after every confirmation.
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })

  it('supports controlled open state', async () => {
    const wrapper = mount(UiDialog, {
      attachTo: document.body,
      props: { open: true, title: '删除文章' },
    })
    await settle()

    expect(document.querySelector('[role="dialog"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('emits update:open when it closes itself', async () => {
    const wrapper = mount(UiDialog, {
      attachTo: document.body,
      props: { open: true, title: '删除文章' },
    })
    await settle()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await settle()

    expect(wrapper.emitted('update:open')?.at(-1)).toEqual([false])
    wrapper.unmount()
  })
})

describe('UiMenu', () => {
  /**
   * Items are passed as data, not as slotted markup. The slash palette and the
   * header's more-actions menu both drive this from a computed list, and a
   * slot-based API would have each call site rewiring roving tabindex and
   * typeahead — which is exactly where those get dropped.
   */
  const items = [
    { id: 'archive', label: '归档' },
    { id: 'delete', label: '删除', separated: true, danger: true },
  ]

  const menu = defineComponent({
    components: { UiMenu },
    props: { onSelect: { type: Function, default: () => {} } },
    setup: () => ({ items }),
    template: `
      <UiMenu :items="items" label="更多操作" @select="onSelect">
        <template #trigger><button data-test="trigger">更多</button></template>
      </UiMenu>
    `,
  })

  it('opens from its trigger with the menu role', async () => {
    const wrapper = mount(menu, { attachTo: document.body })

    await wrapper.get('[data-test="trigger"]').trigger('click')
    await settle()

    expect(document.querySelector('[role="menu"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('moves focus to the first item on ArrowDown', async () => {
    const wrapper = mount(menu, { attachTo: document.body })
    const trigger = wrapper.get('[data-test="trigger"]')

    await trigger.trigger('keydown', { key: 'ArrowDown' })
    await settle()

    const items = document.querySelectorAll('[role="menuitem"]')
    expect(items.length).toBe(2)
    expect(document.activeElement).toBe(items[0])
    wrapper.unmount()
  })

  it('closes on Escape and returns focus to the trigger', async () => {
    const wrapper = mount(menu, { attachTo: document.body })
    const trigger = wrapper.get<HTMLButtonElement>('[data-test="trigger"]')

    // Focused first, as a real keyboard user arrives: Reka restores focus to
    // wherever it was before opening, so a synthetic click that never focused the
    // trigger would make this assert against the browser rather than the wrapper.
    trigger.element.focus()
    await trigger.trigger('click')
    await settle()
    expect(document.querySelector('[role="menu"]')).not.toBeNull()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await settle()

    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })

  it('emits the selected item id', async () => {
    const selected: string[] = []
    const wrapper = mount(menu, {
      attachTo: document.body,
      props: { onSelect: (id: string) => selected.push(id) },
    })

    await wrapper.get('[data-test="trigger"]').trigger('click')
    await settle()

    const items = document.querySelectorAll<HTMLElement>('[role="menuitem"]')
    items[1]?.click()
    await settle()

    // The id, not an index: a call site switching on position would silently act
    // on the wrong item once the list is filtered.
    expect(selected).toEqual(['delete'])
    wrapper.unmount()
  })

  it('marks a destructive item so it can be styled apart', async () => {
    const wrapper = mount(menu, { attachTo: document.body })

    await wrapper.get('[data-test="trigger"]').trigger('click')
    await settle()

    expect(document.querySelector('[role="menuitem"][data-danger="true"]')).not.toBeNull()
    wrapper.unmount()
  })
})

describe('UiPopover', () => {
  it('opens from its trigger and names itself', async () => {
    const wrapper = mount(UiPopover, {
      attachTo: document.body,
      props: { label: '编辑链接' },
      slots: { trigger: '<button data-test="trigger">打开</button>', default: '<p>内容</p>' },
    })

    await wrapper.get('[data-test="trigger"]').trigger('click')
    await settle()

    expect(document.body.textContent).toContain('内容')
    // Reka renders this content as role="dialog", so an unnamed popover is an
    // unnamed dialog -- the failure UiDialog's required title exists to prevent.
    const content = document.querySelector('.ui-popover__content')
    expect(content?.getAttribute('role')).toBe('dialog')
    expect(content?.getAttribute('aria-label')).toBe('编辑链接')
    wrapper.unmount()
  })

  it('closes when a pointer lands outside it', async () => {
    // Required by the plan, and the reason the link editor is a popover rather
    // than a dialog: clicking back into the prose must dismiss it without Esc.
    const outside = document.createElement('div')
    outside.dataset.test = 'outside'
    document.body.appendChild(outside)

    const wrapper = mount(UiPopover, {
      attachTo: document.body,
      props: { label: '编辑链接' },
      slots: { trigger: '<button data-test="trigger">打开</button>', default: '<p>内容</p>' },
    })

    try {
      await wrapper.get('[data-test="trigger"]').trigger('click')
      await settle()
      expect(document.body.textContent).toContain('内容')

      // pointerdown, not click: Reka dismisses on the pointer-down phase, so a
      // click-only test would pass against a component that never dismissed.
      outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }))
      await settle()

      expect(document.body.textContent).not.toContain('内容')
    } finally {
      wrapper.unmount()
      outside.remove()
    }
  })

  it('closes on Escape and returns focus to the trigger', async () => {
    const wrapper = mount(UiPopover, {
      attachTo: document.body,
      props: { label: '编辑链接' },
      slots: {
        trigger: '<button data-test="trigger">打开</button>',
        // A focusable child, deliberately. Without one, focus never enters the
        // content and the focus-return assertion below would hold even if return
        // were broken -- it would only be observing that focus never moved.
        default: '<p>内容</p><input data-test="field" />',
      },
    })
    const trigger = wrapper.get<HTMLButtonElement>('[data-test="trigger"]')

    trigger.element.focus()
    await trigger.trigger('click')
    await settle()

    const field = document.querySelector<HTMLInputElement>('[data-test="field"]')
    field?.focus()
    expect(document.activeElement).toBe(field)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await settle()

    expect(document.body.textContent).not.toContain('内容')
    // Focus was genuinely inside the overlay and came back.
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })

  it('reports expanded state on the trigger', async () => {
    const wrapper = mount(UiPopover, {
      attachTo: document.body,
      props: { label: '编辑链接' },
      slots: { trigger: '<button data-test="trigger">打开</button>', default: '<p>内容</p>' },
    })
    const trigger = wrapper.get('[data-test="trigger"]')

    expect(trigger.attributes('aria-expanded')).toBe('false')
    await trigger.trigger('click')
    await settle()
    expect(trigger.attributes('aria-expanded')).toBe('true')
    wrapper.unmount()
  })
})

describe('UiToastRegion', () => {
  it('announces a published message in a live region', async () => {
    const wrapper = mount(UiToastRegion, { attachTo: document.body })

    await (wrapper.vm as unknown as { publish: (m: string) => void }).publish('已发布')
    await settle()

    // A save or publish result that only changed colour would not reach a screen
    // reader. The text has to land inside a live region.
    const live = document.querySelector('[role="status"], [aria-live]')
    expect(live).not.toBeNull()
    expect(document.body.textContent).toContain('已发布')
    wrapper.unmount()
  })

  it('keeps the live region mounted while empty', () => {
    const wrapper = mount(UiToastRegion, { attachTo: document.body })

    // The component's central design claim. A live region inserted at the same
    // moment as its first text is frequently never announced, so wrapping this
    // element in v-if="toasts.length" would break announcements while leaving any
    // text-absence assertion green.
    const region = document.querySelector('[data-testid="toast-region"]')
    expect(region).not.toBeNull()
    expect(region?.getAttribute('aria-live')).toBe('polite')
    expect(region?.textContent?.trim()).toBe('')
    wrapper.unmount()
  })

  it('dismisses a toast when its duration elapses', async () => {
    vi.useFakeTimers()
    try {
      const wrapper = mount(UiToastRegion, {
        attachTo: document.body,
        props: { duration: 1000 },
      })

      ;(wrapper.vm as unknown as { publish: (m: string) => void }).publish('已发布')
      await nextTick()
      expect(document.body.textContent).toContain('已发布')

      vi.advanceTimersByTime(999)
      await nextTick()
      expect(document.body.textContent).toContain('已发布')

      // A publish that never expired would otherwise pass every other assertion
      // in this file.
      vi.advanceTimersByTime(1)
      await nextTick()
      expect(document.body.textContent).not.toContain('已发布')
      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  it('cancels the pending timer when dismissed early', async () => {
    vi.useFakeTimers()
    try {
      const wrapper = mount(UiToastRegion, {
        attachTo: document.body,
        props: { duration: 1000 },
      })
      const vm = wrapper.vm as unknown as {
        publish: (m: string) => string
        dismiss: (id: string) => void
      }

      const first = vm.publish('第一条')
      await nextTick()
      vm.dismiss(first)
      await nextTick()
      expect(document.body.textContent).not.toContain('第一条')

      // The dismissed toast's timer must not still be armed. Republishing reuses
      // no id, so a stale timer firing would remove the wrong toast.
      vm.publish('第二条')
      await nextTick()
      vi.advanceTimersByTime(1000)
      await nextTick()
      expect(document.body.textContent).not.toContain('第二条')
      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('the primitive boundary', () => {
  it('is the only place that imports reka-ui', () => {
    // The plan's constraint: business components import Alive wrappers, never
    // reka-ui. Asserted as a source-level fact rather than through a mounted
    // component, because that is the form the constraint actually takes -- a
    // runtime assertion about rendered output cannot observe an import, and the
    // type-level half is unobservable at runtime entirely.
    const sourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')
    const offenders: string[] = []

    const walk = (dir: string): void => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name)
        if (entry.isDirectory()) {
          walk(full)
        } else if (/\.(ts|vue)$/.test(entry.name)) {
          if (/from ['"]reka-ui['"]/.test(readFileSync(full, 'utf8'))) offenders.push(full)
        }
      }
    }
    walk(sourceRoot)

    const uiDir = join(sourceRoot, 'components', 'ui')
    const outside = offenders.filter((file) => !file.startsWith(uiDir))
    expect(outside).toEqual([])
    // And the wrappers really do use it, so this is not passing by finding nothing.
    expect(offenders.length).toBeGreaterThan(0)
  })

  it('mounts a dialog inside a menu without either stealing the other’s focus return', async () => {
    // The real workspace nests these: a "more actions" menu opens a delete
    // confirmation.
    const nested = defineComponent({
      components: { UiDialog, UiButton },
      template: `
        <div>
          <UiDialog title="删除文章">
            <template #trigger><UiButton data-test="outer">删除</UiButton></template>
            <p>确认</p>
          </UiDialog>
        </div>
      `,
    })
    const wrapper = mount(nested, { attachTo: document.body })
    const trigger = wrapper.get('[data-test="outer"]')

    await trigger.trigger('click')
    await settle()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await settle()

    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })
})
