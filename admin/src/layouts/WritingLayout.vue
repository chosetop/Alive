<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref } from 'vue'

import ArticleDirectory from '../components/writing/ArticleDirectory.vue'
import { UiDialog } from '../components/ui'
import { resolveAdminWorld } from '../content-worlds/registry'
import { getSiteSettings } from '../api/site'
import { useThemeStore } from '../stores/theme'
import { useWritingStore, writingFlushKey, type WritingFlushGate } from '../stores/writing'

/**
 * The writing shell: directory, canvas, and nothing between them.
 *
 * This is a sibling of `AdminLayout`, not a child. The utility shell's header and
 * nav sidebar are the form-first chrome this workspace exists to get out of the
 * way; nesting would put both on screen and leave the canvas competing with a
 * navigation rail for the same attention. Authentication is not repeated here --
 * both top-level records carry `requiresAuth` and the router's single guard is the
 * only place that decision is made.
 *
 * Below the drawer breakpoint the directory becomes a modal dialog rather than a
 * narrower column. A 14rem pane beside a canvas at 375px leaves neither usable,
 * and a modal is the one presentation where the canvas keeps its full width and
 * the pane keeps its full content.
 */

/**
 * 48rem, matching the plan. Read once through `matchMedia` rather than by
 * watching resize: a resize listener fires on every scroll-driven viewport change
 * on iOS Safari, and this only needs the two states.
 */
const DRAWER_QUERY = '(max-width: 48rem)'
const MIN_DIRECTORY_WIDTH = 220
const MAX_DIRECTORY_WIDTH = 420
const DEFAULT_DIRECTORY_WIDTH = 224
const DIRECTORY_WIDTH_STORAGE_KEY = 'alive:writing-directory-width'

const writing = useWritingStore()
const theme = useThemeStore()
const currentDirectoryTitle = computed(
  () => `${resolveAdminWorld(writing.activeWorld)?.directoryNoun ?? '世界'}目录`,
)

const isNarrow = ref(false)
const directoryWidth = ref(DEFAULT_DIRECTORY_WIDTH)
let mediaQuery: MediaQueryList | null = null
let resizeStartX = 0
let resizeStartWidth = DEFAULT_DIRECTORY_WIDTH
let resizing = false

/**
 * The gate the directory pulls before it navigates. The editor registers itself
 * here on mount; it is null whenever no editor is mounted, which is a real state
 * -- the drawer can be open over a canvas that has not loaded an article yet.
 */
const flushGate: WritingFlushGate = ref(null)
provide(writingFlushKey, flushGate)

function applyMatch(matches: boolean): void {
  isNarrow.value = matches
  // Entering the narrow range with the directory open would leave a modal over
  // the canvas nobody asked to open. Leaving it re-opens the column, because at
  // desktop width the pane is the default and a collapse taken to dodge a
  // cramped layout should not persist past the reason for it.
  writing.setDirectoryOpen(!matches)
}

function clampDirectoryWidth(width: number): number {
  return Math.min(MAX_DIRECTORY_WIDTH, Math.max(MIN_DIRECTORY_WIDTH, width))
}

function readDirectoryWidth(): void {
  try {
    const stored = Number(window.localStorage.getItem(DIRECTORY_WIDTH_STORAGE_KEY))
    if (Number.isFinite(stored)) directoryWidth.value = clampDirectoryWidth(stored)
  } catch {
    // A restricted storage context should not prevent the writing shell from opening.
  }
}

function persistDirectoryWidth(): void {
  try {
    window.localStorage.setItem(DIRECTORY_WIDTH_STORAGE_KEY, String(directoryWidth.value))
  } catch {
    // Persistence is a convenience, not a prerequisite for resizing.
  }
}

function stopResize(): void {
  if (!resizing) return
  resizing = false
  window.removeEventListener('pointermove', resizeDirectory)
  window.removeEventListener('pointerup', stopResize)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  persistDirectoryWidth()
}

function resizeDirectory(event: PointerEvent): void {
  if (!resizing) return
  directoryWidth.value = clampDirectoryWidth(resizeStartWidth + event.clientX - resizeStartX)
}

function startResize(event: PointerEvent): void {
  if (isNarrow.value || !writing.directoryOpen) return
  resizing = true
  resizeStartX = event.clientX
  resizeStartWidth = directoryWidth.value
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  window.addEventListener('pointermove', resizeDirectory)
  window.addEventListener('pointerup', stopResize)
}

function resizeByKeyboard(delta: number): void {
  directoryWidth.value = clampDirectoryWidth(directoryWidth.value + delta)
  persistDirectoryWidth()
}

function handleResizeKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    resizeByKeyboard(-16)
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    resizeByKeyboard(16)
  } else if (event.key === 'Home') {
    event.preventDefault()
    directoryWidth.value = MIN_DIRECTORY_WIDTH
    persistDirectoryWidth()
  } else if (event.key === 'End') {
    event.preventDefault()
    directoryWidth.value = MAX_DIRECTORY_WIDTH
    persistDirectoryWidth()
  }
}

onMounted(() => {
  readDirectoryWidth()
  void loadTheme()
  // jsdom has no matchMedia unless a test installs one. Absent, the workspace
  // stays in its desktop layout, which is the correct fallback: a two-column
  // grid degrades to a wide directory, while assuming narrow would hide the
  // directory behind a modal on a desktop that never asked for one.
  if (typeof window.matchMedia !== 'function') return

  mediaQuery = window.matchMedia(DRAWER_QUERY)
  applyMatch(mediaQuery.matches)
  mediaQuery.addEventListener('change', handleMediaChange)
})

async function loadTheme(): Promise<void> {
  try {
    theme.hydrate(await getSiteSettings())
  } catch {
    // Ink is the shared fallback; a settings outage must not block writing.
  }
}

onBeforeUnmount(() => {
  stopResize()
  mediaQuery?.removeEventListener('change', handleMediaChange)
  mediaQuery = null
})

function handleMediaChange(event: MediaQueryListEvent): void {
  applyMatch(event.matches)
}
</script>

<template>
  <div
    class="workspace"
    data-writing-workspace
    data-visual-mode="writing-desk"
    :style="{ '--directory-width': `${directoryWidth}px` }"
    :data-directory-open="writing.directoryOpen && !isNarrow && writing.activeWorld ? 'true' : 'false'"
  >
    <!--
      Two mount points, one component, and only ever one mounted. Rendering the
      column and the drawer at once would put two copies of every article row in
      the accessibility tree and two identical ids on the search input.
    -->
    <template v-if="writing.activeWorld">
      <div v-if="!isNarrow" class="rail" :data-collapsed="writing.directoryOpen ? 'false' : 'true'">
        <ArticleDirectory
          v-if="writing.directoryOpen"
          :key="writing.activeWorld"
          :world="writing.activeWorld"
        />
        <button
          v-if="writing.directoryOpen"
          class="rail-resize"
          type="button"
          role="separator"
          aria-label="调整文章目录宽度"
          :aria-valuemin="MIN_DIRECTORY_WIDTH"
          :aria-valuemax="MAX_DIRECTORY_WIDTH"
          :aria-valuenow="directoryWidth"
          data-directory-resize
          @pointerdown.prevent="startResize"
          @keydown="handleResizeKeydown"
        />
      </div>

      <UiDialog
        v-else
        :title="currentDirectoryTitle"
        hide-title
        :open="writing.directoryOpen"
        @update:open="writing.setDirectoryOpen($event)"
      >
        <div class="writing-drawer" data-writing-drawer>
          <ArticleDirectory
            :key="writing.activeWorld"
            :world="writing.activeWorld"
            drawer
          />
        </div>
      </UiDialog>
    </template>

    <main class="canvas">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.workspace {
  display: grid;
  /* The directory column is a grid track rather than a flex sibling so that
     collapsing it to 0 lets the canvas recentre in the same layout pass. A flex
     child animating to width:0 keeps its padding and its border in the flow. */
  grid-template-columns: var(--directory-width, 14rem) minmax(0, 1fr);
  height: 100vh;
  overflow: hidden;
  background: var(--c-surface-sunken);
}

.workspace[data-directory-open='false'] {
  grid-template-columns: 0 minmax(0, 1fr);
}

.rail {
  position: relative;
  min-width: 0;
  border-right: 1px solid var(--c-line);
  overflow: hidden;
  background: var(--c-glass);
  box-shadow: var(--shadow-control);
}

.rail-resize {
  position: absolute;
  z-index: 2;
  top: 0;
  right: -0.25rem;
  bottom: 0;
  width: 0.5rem;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: col-resize;
  transition: background-color var(--motion-fast) ease;
}

.rail-resize:hover,
.rail-resize:focus-visible {
  background: color-mix(in srgb, var(--c-accent) 35%, transparent);
}

/* Border and all: a collapsed rail that kept its 1px line would leave a stripe
   down the left of a canvas that is meant to be uninterrupted. */
.rail[data-collapsed='true'] {
  border-right: none;
}

.canvas {
  min-width: 0;
  height: 100%;
  overflow-x: clip;
  overflow-y: auto;
  background: var(--c-paper);
}

.writing-drawer {
  height: 100%;
  min-height: 0;
}

/* UiDialog owns focus trapping and Escape. This selector changes only the
   writing directory's presentation from a centred modal to a left drawer. */
:global(.ui-dialog__content:has([data-writing-drawer])) {
  inset-block: 0;
  inset-inline: 0 auto;
  width: min(22rem, calc(100vw - var(--space-5)));
  max-height: 100dvh;
  padding: 0;
  overflow: hidden;
  transform: none;
  border-block: 0;
  border-inline-start: 0;
  border-radius: 0 var(--radius-surface) var(--radius-surface) 0;
  overscroll-behavior: contain;
  animation: writing-drawer-enter var(--motion-fast) cubic-bezier(0.16, 1, 0.3, 1);
}

:global(.ui-dialog__body:has([data-writing-drawer])) {
  height: 100%;
  min-height: 0;
}

@keyframes writing-drawer-enter {
  from {
    opacity: 0;
    transform: translateX(-1rem);
  }

  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@media (pointer: coarse) {
  .rail-resize {
    width: 0.875rem;
    right: -0.4375rem;
  }
}

@media (max-width: 48rem) {
  /* The drawer is a modal, so there is no directory track to reserve. */
  .workspace,
  .workspace[data-directory-open='false'] {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 23.4375rem) {
  .workspace,
  .canvas {
    max-width: 100vw;
    overflow-x: clip;
  }

  :global(.ui-dialog__content:has([data-writing-drawer])) {
    width: calc(100vw - var(--space-4));
    max-width: 100vw;
  }
}

@media (prefers-reduced-motion: reduce) {
  .rail-resize {
    transition: none;
  }

  :global(.ui-dialog__content:has([data-writing-drawer])) {
    animation: none;
  }
}
</style>
