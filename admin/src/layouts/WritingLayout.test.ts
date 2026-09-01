import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, inject, nextTick } from 'vue'

import { useWritingStore, writingFlushKey, type WritingFlushGate } from '../stores/writing'
import WritingLayout from './WritingLayout.vue'

const transition = vi.hoisted(() => ({
  dispose: vi.fn(),
  enterWorld: vi.fn(),
  enterWorkspace: vi.fn(),
  swapCanvas: vi.fn(),
}))

/**
 * The shell's geometry and its two presentations of the directory. What matters
 * here is that exactly one directory exists at a time and that the canvas track
 * actually collapses -- both are things a plausible refactor breaks while every
 * screenshot still looks acceptable.
 */

const media = {
  matches: false,
  listeners: new Set<(event: MediaQueryListEvent) => void>(),
}

/** The directory does real API work; the layout's own behaviour does not need it. */
vi.mock('../components/writing/ArticleDirectory.vue', () => ({
  default: defineComponent({
    name: 'ArticleDirectory',
    props: {
      drawer: { type: Boolean, default: false },
      world: { type: String, required: true },
    },
    template: '<nav data-stub-directory :data-drawer="drawer ? \'true\' : \'false\'" :data-world="world" />',
  }),
}))

vi.mock('../composables/useWorldTransition', () => ({
  useWorldTransition: () => transition,
  worldTransitionKey: Symbol('world-transition'),
  writingRailKey: Symbol('writing-rail'),
}))

const wrappers: VueWrapper[] = []

function installMatchMedia(matches: boolean): void {
  media.matches = matches
  media.listeners.clear()
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      media: query,
      get matches() {
        return media.matches
      },
      addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => {
        media.listeners.add(listener)
      },
      removeEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => {
        media.listeners.delete(listener)
      },
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
      onchange: null,
    }),
  })
}

/** Drives the media query the way a real viewport change would. */
async function setNarrow(matches: boolean): Promise<void> {
  media.matches = matches
  for (const listener of media.listeners) {
    listener({ matches } as MediaQueryListEvent)
  }
  await nextTick()
}

async function mountLayout(): Promise<VueWrapper> {
  const wrapper = mount(WritingLayout, {
    global: {
      plugins: [createPinia()],
      stubs: { RouterView: { template: '<div data-stub-canvas />' } },
    },
    attachTo: document.body,
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('WritingLayout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    installMatchMedia(false)
    transition.dispose.mockReset()
    transition.enterWorld.mockReset()
    transition.enterWorkspace.mockReset()
    transition.swapCanvas.mockReset()
  })

  afterEach(async () => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
    await flushPromises()
  })

  it('renders the directory as a column at desktop width', async () => {
    const wrapper = await mountLayout()

    expect(wrapper.get('[data-writing-workspace]').attributes('data-directory-open')).toBe('false')
    expect(wrapper.find('[data-stub-directory]').exists()).toBe(false)
    useWritingStore().setActiveWorld('journal')
    await nextTick()
    expect(wrapper.get('[data-stub-directory]').attributes('data-drawer')).toBe('false')
    expect(wrapper.get('[data-stub-directory]').attributes('data-world')).toBe('journal')
    expect(wrapper.get('[data-stub-canvas]').element).toBeTruthy()
  })

  it('collapses the directory track and keeps the canvas', async () => {
    const wrapper = await mountLayout()
    const store = useWritingStore()

    store.setDirectoryOpen(false)
    await nextTick()

    // The attribute is what the grid rule reads to zero the track, so this is the
    // observable half of "the canvas recentres" in an environment with no layout.
    expect(wrapper.get('[data-writing-workspace]').attributes('data-directory-open')).toBe('false')
    expect(wrapper.find('[data-stub-directory]').exists()).toBe(false)
    expect(wrapper.get('[data-stub-canvas]').element).toBeTruthy()
  })

  it('mounts one directory at a time, never both presentations', async () => {
    const wrapper = await mountLayout()
    useWritingStore().setActiveWorld('journal')
    await nextTick()
    expect(wrapper.findAll('[data-stub-directory]')).toHaveLength(1)

    await setNarrow(true)
    useWritingStore().setDirectoryOpen(true)
    await flushPromises()

    // Two copies would put every article row in the accessibility tree twice and
    // duplicate the search input's id, which silently breaks its label.
    expect(document.querySelectorAll('[data-stub-directory]')).toHaveLength(1)
  })

  it('closes the directory on entering the narrow range and reopens on leaving', async () => {
    const wrapper = await mountLayout()
    const store = useWritingStore()
    store.setActiveWorld('journal')
    await nextTick()
    expect(store.directoryOpen).toBe(true)

    await setNarrow(true)
    // Not left open: it would land as a modal over a canvas nobody asked to cover.
    expect(store.directoryOpen).toBe(false)
    expect(wrapper.get('[data-writing-workspace]').attributes('data-directory-open')).toBe('false')

    await setNarrow(false)
    expect(store.directoryOpen).toBe(true)
  })

  it('starts closed when the viewport is already narrow at mount', async () => {
    installMatchMedia(true)
    await mountLayout()

    expect(useWritingStore().directoryOpen).toBe(false)
  })

  it('presents the narrow directory as a modal marked as a drawer', async () => {
    installMatchMedia(true)
    await mountLayout()
    useWritingStore().setActiveWorld('video')
    useWritingStore().setDirectoryOpen(true)
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))
    await nextTick()

    // role="dialog" plus a name that resolves to actual text. Asserting only that
    // aria-labelledby is present is a false pass: the attribute is there even when
    // the title is empty, which is precisely the unnamed dialog UiDialog's required
    // title exists to prevent.
    const dialog = document.querySelector('[role="dialog"]')
    expect(dialog).not.toBeNull()
    const labelId = dialog?.getAttribute('aria-labelledby')
    expect(labelId).toBeTruthy()
    expect(document.getElementById(labelId as string)?.textContent?.trim()).toBe('影像目录')
    expect(document.querySelector('[data-stub-directory]')?.getAttribute('data-drawer')).toBe(
      'true',
    )
    expect(document.querySelector('[data-stub-directory]')?.getAttribute('data-world')).toBe('video')
  })

  it('stays in the desktop layout when matchMedia is unavailable', async () => {
    // The correct fallback: a two-column grid degrades to a wide directory, while
    // assuming narrow would hide the pane behind a modal on a desktop.
    Reflect.deleteProperty(window, 'matchMedia')
    const wrapper = await mountLayout()

    expect(wrapper.get('[data-writing-workspace]').attributes('data-directory-open')).toBe('false')
    expect(wrapper.find('[data-stub-directory]').exists()).toBe(false)
  })

  it('provides a flush gate that starts empty', async () => {
    let received: WritingFlushGate | undefined
    const probe = defineComponent({
      setup() {
        received = inject(writingFlushKey)
        return () => null
      },
    })
    const wrapper = mount(WritingLayout, {
      global: {
        plugins: [createPinia()],
        stubs: { RouterView: probe },
      },
    })
    wrappers.push(wrapper)
    await flushPromises()

    // Provided, and null until an editor registers. Not providing it at all would
    // make the directory fall back to navigating without a flush -- losing the last
    // keystroke on every article switch, silently.
    expect(received).toBeDefined()
    expect(received?.value).toBeNull()
  })

  it('removes its media listener on unmount', async () => {
    const wrapper = await mountLayout()
    expect(media.listeners.size).toBe(1)

    wrapper.unmount()
    wrappers.splice(wrappers.indexOf(wrapper), 1)

    // A layout that leaked one listener per mount would rewrite the store's
    // directory state from a dead component on the next viewport change.
    expect(media.listeners.size).toBe(0)
  })

  it('disposes the provided world transition on unmount', async () => {
    const wrapper = await mountLayout()

    wrapper.unmount()
    wrappers.splice(wrappers.indexOf(wrapper), 1)

    expect(transition.dispose).toHaveBeenCalledOnce()
  })

  it('zeroes the directory track rather than hiding its contents', () => {
    // The collapse has to remove the column from the grid. Hiding the pane while
    // leaving a 14rem track would leave the canvas off-centre with an empty gutter,
    // which is the opposite of the recentring the plan asks for.
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'WritingLayout.vue'),
      'utf8',
    )
    const collapsed = source.match(/\[data-directory-open='false'\] \{[^}]*\}/)?.[0] ?? ''

    expect(collapsed).toMatch(/grid-template-columns:\s*0\s/)
  })

  it('renders a keyboard-accessible rail resize handle with bounded width tokens', () => {
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'WritingLayout.vue'),
      'utf8',
    )

    expect(source).toContain('data-directory-resize')
    expect(source).toContain('--directory-width')
    expect(source).toContain('MIN_DIRECTORY_WIDTH')
    expect(source).toContain('MAX_DIRECTORY_WIDTH')
  })

  it('identifies the writing desk and exposes the real directory resize bounds', async () => {
    const wrapper = await mountLayout()
    useWritingStore().setActiveWorld('journal')
    await nextTick()
    const workspace = wrapper.get('[data-writing-workspace]')
    const resize = wrapper.get('[data-directory-resize]')

    expect(workspace.attributes('data-visual-mode')).toBe('writing-desk')
    expect(resize.attributes('role')).toBe('separator')
    expect(resize.attributes('aria-valuemin')).toBe('220')
    expect(resize.attributes('aria-valuemax')).toBe('420')
  })

  it('gates and keys the directory by the active world', () => {
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'WritingLayout.vue'),
      'utf8',
    )

    expect(source).toContain('v-if="writing.activeWorld"')
    expect(source).toContain(':key="writing.activeWorld"')
    expect(source).toContain(':world="writing.activeWorld"')
  })

  it('defines a 375px no-overflow contract and a reduced-motion fallback', () => {
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'WritingLayout.vue'),
      'utf8',
    )

    expect(source).toMatch(/@media \(max-width:\s*23\.4375rem\)[\s\S]*overflow-x:\s*clip/)
    expect(source).toMatch(/@media \(prefers-reduced-motion:\s*reduce\)[\s\S]*animation:\s*none/)
  })
})
